<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let status = $state(null)
  let loading = $state(true)
  let saving = $state(false)
  let error = $state('')
  let msg = $state('')
  let tab = $state('connections')
  let editIdx = $state(0)

  let dns = $state('')
  let connections = $state([])

  onMount(() => { load() })

  async function load() {
    loading = true
    error = ''
    try {
      status = await api.network()
      dns = (status.dns || []).join(', ')
      connections = structuredClone(status.connections || [])
      if (connections.length && editIdx >= connections.length) editIdx = 0
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  function blankConn(type) {
    const base = {
      name: '',
      type,
      ipv4_method: 'dhcp',
      ipv6_method: 'dhcp',
      dns: [],
    }
    if (type === 'bond') {
      return { ...base, name: 'bond0', bond_mode: '802.3ad', slaves: [] }
    }
    if (type === 'vlan') {
      return { ...base, name: 'vlan100', vlan_id: 100, parent: status?.interfaces?.[0]?.name || '' }
    }
    return { ...base, name: status?.interfaces?.[0]?.name || '' }
  }

  function addConn(type) {
    connections = [...connections, blankConn(type)]
    editIdx = connections.length - 1
    tab = 'connections'
  }

  function removeConn(i) {
    if (!confirm(`Supprimer ${connections[i].name || 'cette connexion'} ?`)) return
    connections = connections.filter((_, j) => j !== i)
    editIdx = Math.max(0, editIdx - 1)
  }

  function toggleSlave(i, iface) {
    const slaves = new Set(connections[i].slaves || [])
    if (slaves.has(iface)) slaves.delete(iface)
    else slaves.add(iface)
    connections[i].slaves = [...slaves]
    connections = [...connections]
  }

  async function save() {
    saving = true
    error = ''
    msg = ''
    try {
      const body = {
        renderer: status?.renderer || 'networkd',
        dns: dns.split(/[,;\s]+/).map((s) => s.trim()).filter(Boolean),
        connections: connections.map((c) => ({
          ...c,
          vlan_id: c.vlan_id ? Number(c.vlan_id) : undefined,
          mtu: c.mtu ? Number(c.mtu) : undefined,
        })),
      }
      status = await api.networkPut(body)
      connections = structuredClone(status.connections || [])
      dns = (status.dns || []).join(', ')
      msg = 'Configuration réseau appliquée (netplan)'
    } catch (e) {
      error = e.message
    } finally {
      saving = false
    }
  }

  async function reapply() {
    try {
      await api.networkApply()
      msg = 'netplan apply exécuté'
      await load()
    } catch (e) {
      error = e.message
    }
  }

</script>

<p class="hint">
  Configuration via <strong>netplan</strong> (systemd-networkd). Écrit <code>/etc/netplan/90-bytebay.yaml</code>.
  LACP = bond mode <code>802.3ad</code>. Attention : une mauvaise config peut couper l'accès réseau.
</p>

{#if loading}
  <p>Chargement…</p>
{:else}
  <div class="tabs">
    <button class:active={tab === 'connections'} onclick={() => (tab = 'connections')}>Connexions</button>
    <button class:active={tab === 'ifaces'} onclick={() => (tab = 'ifaces')}>Interfaces</button>
    <button class="ghost" onclick={reapply}>Réappliquer</button>
  </div>

  <label class="dns-row">DNS global
    <input placeholder="8.8.8.8, 1.1.1.1" bind:value={dns} />
  </label>

  {#if tab === 'ifaces'}
    <table>
      <thead><tr><th>Interface</th><th>MAC</th><th>État</th><th>Maître</th></tr></thead>
      <tbody>
        {#each status?.interfaces || [] as iface}
          <tr>
            <td><code>{iface.name}</code></td>
            <td class="muted">{iface.mac}</td>
            <td>{iface.state}{#if iface.speed} · {iface.speed} Mb/s{/if}</td>
            <td>{iface.master || '—'}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {:else}
    <div class="toolbar">
      <button class="ghost" onclick={() => addConn('ethernet')}>+ Ethernet</button>
      <button class="ghost" onclick={() => addConn('bond')}>+ LACP (bond)</button>
      <button class="ghost" onclick={() => addConn('vlan')}>+ VLAN</button>
    </div>

    <div class="layout">
      <ul class="conn-list">
        {#each connections as c, i}
          <li>
            <button class:sel={editIdx === i} onclick={() => (editIdx = i)}>
              <span class="type">{c.type || 'ethernet'}</span>
              <strong>{c.name || '—'}</strong>
              {#if c.addresses?.length}
                <span class="muted">{c.addresses[0]}</span>
              {/if}
            </button>
          </li>
        {/each}
      </ul>

      {#if connections[editIdx]}
        <div class="editor">
          <div class="row2">
            <label>Nom
              <input bind:value={connections[editIdx].name} />
            </label>
            <label>Type
              <select bind:value={connections[editIdx].type}>
                <option value="ethernet">Ethernet</option>
                <option value="bond">Bond / LACP</option>
                <option value="vlan">VLAN</option>
              </select>
            </label>
          </div>

          {#if connections[editIdx].type === 'bond'}
            <label>Mode LACP
              <select bind:value={connections[editIdx].bond_mode}>
                <option value="802.3ad">802.3ad (LACP)</option>
                <option value="active-backup">active-backup</option>
                <option value="balance-rr">balance-rr</option>
                <option value="balance-xor">balance-xor</option>
                <option value="broadcast">broadcast</option>
              </select>
            </label>
            <p class="lbl">Esclaves</p>
            <div class="slaves">
              {#each status?.interfaces || [] as iface}
                <label class="chk">
                  <input
                    type="checkbox"
                    checked={(connections[editIdx].slaves || []).includes(iface.name)}
                    onchange={() => toggleSlave(editIdx, iface.name)}
                  />
                  {iface.name}
                </label>
              {/each}
            </div>
          {/if}

          {#if connections[editIdx].type === 'vlan'}
            <div class="row2">
              <label>VLAN ID
                <input type="number" min="1" max="4094" bind:value={connections[editIdx].vlan_id} />
              </label>
              <label>Interface parente
                <select bind:value={connections[editIdx].parent}>
                  {#each status?.interfaces || [] as iface}
                    <option value={iface.name}>{iface.name}</option>
                  {/each}
                </select>
              </label>
            </div>
          {/if}

          <h4>IPv4</h4>
          <div class="row2">
            <label>Mode
              <select bind:value={connections[editIdx].ipv4_method}>
                <option value="dhcp">DHCP</option>
                <option value="static">Statique</option>
                <option value="disabled">Désactivé</option>
              </select>
            </label>
            {#if connections[editIdx].ipv4_method === 'static'}
              <label>Adresse
                <input placeholder="192.168.1.10/24" bind:value={connections[editIdx].ipv4_address} />
              </label>
              <label>Passerelle
                <input placeholder="192.168.1.1" bind:value={connections[editIdx].ipv4_gateway} />
              </label>
            {/if}
          </div>

          <h4>IPv6</h4>
          <div class="row2">
            <label>Mode
              <select bind:value={connections[editIdx].ipv6_method}>
                <option value="dhcp">DHCP</option>
                <option value="auto">Auto (SLAAC)</option>
                <option value="static">Statique</option>
                <option value="disabled">Désactivé</option>
              </select>
            </label>
            {#if connections[editIdx].ipv6_method === 'static'}
              <label>Adresse
                <input placeholder="fd00::1/64" bind:value={connections[editIdx].ipv6_address} />
              </label>
              <label>Passerelle
                <input placeholder="fd00::1" bind:value={connections[editIdx].ipv6_gateway} />
              </label>
            {/if}
          </div>

          <label>DNS (connexion)
            <input
              placeholder="Laisser vide pour DNS global"
              value={(connections[editIdx].dns || []).join(', ')}
              oninput={(e) => {
                connections[editIdx].dns = e.currentTarget.value.split(/[,;\s]+/).map((s) => s.trim()).filter(Boolean)
                connections = [...connections]
              }}
            />
          </label>

          <label>MTU
            <input type="number" placeholder="1500" bind:value={connections[editIdx].mtu} />
          </label>

          {#if connections[editIdx].addresses?.length}
            <p class="live">Actif : {connections[editIdx].addresses.join(', ')} ({connections[editIdx].oper_state})</p>
          {/if}

          <button class="danger ghost" onclick={() => removeConn(editIdx)}>Supprimer</button>
        </div>
      {/if}
    </div>

    <button onclick={save} disabled={saving || !connections.length}>
      {saving ? 'Application…' : 'Enregistrer et appliquer'}
    </button>
  {/if}
{/if}

{#if msg}<p class="ok">{msg}</p>{/if}
{#if error}<p class="err">{error}</p>{/if}

<style>
  .hint { color: var(--bb-muted); font-size: 11px; margin-bottom: 10px; }
  .tabs { display: flex; gap: 6px; margin-bottom: 10px; flex-wrap: wrap; }
  .tabs button.active { background: var(--bb-accent, #4a9eff); color: #fff; }
  .dns-row { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--bb-muted); margin-bottom: 12px; }
  .toolbar { display: flex; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
  .layout { display: grid; grid-template-columns: 160px 1fr; gap: 12px; margin-bottom: 12px; min-height: 280px; }
  .conn-list { list-style: none; margin: 0; padding: 0; }
  .conn-list button {
    width: 100%; text-align: left; padding: 8px; margin-bottom: 4px;
    border: 1px solid var(--bb-border); border-radius: 6px; background: var(--bb-panel);
    display: flex; flex-direction: column; gap: 2px; cursor: pointer;
  }
  .conn-list button.sel { border-color: var(--bb-accent, #4a9eff); }
  .type { font-size: 9px; text-transform: uppercase; color: var(--bb-muted); }
  .muted { font-size: 10px; color: var(--bb-muted); }
  .editor { border: 1px solid var(--bb-border); border-radius: 8px; padding: 12px; overflow: auto; }
  .row2 { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 8px; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--bb-muted); margin-bottom: 6px; }
  h4 { font-size: 11px; color: var(--bb-muted); margin: 10px 0 6px; text-transform: uppercase; }
  .lbl { font-size: 11px; color: var(--bb-muted); margin: 6px 0 4px; }
  .slaves { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 8px; }
  .chk { flex-direction: row; align-items: center; gap: 6px; color: var(--bb-text); }
  .chk input { width: auto; }
  .live { font-size: 11px; color: var(--bb-ok); margin: 8px 0; }
  .ok { color: var(--bb-ok); margin-top: 8px; }
  .err { color: var(--bb-danger); margin-top: 8px; }
  code { font-size: 11px; }
</style>
