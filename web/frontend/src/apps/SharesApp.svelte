<script>
  import { api } from '../lib/api.js'

  let tab = $state('nfs')
  let cfg = $state({ nfs: [], samba: [], ftp: [] })
  let msg = $state('')
  let err = $state('')
  let applying = $state(false)

  const empty = {
    nfs: { path: '/volumes/raid1/public', clients: '192.168.0.0/16', options: 'rw,sync,no_subtree_check', enabled: true },
    samba: { name: 'public', path: '/volumes/raid1/public', browseable: true, read_only: false, guest_ok: false, enabled: true },
    ftp: { name: 'uploads', path: '/volumes/raid1/ftp', enabled: true },
  }

  $effect(() => { load() })

  async function load() {
    try {
      cfg = await api.shares()
      for (const k of ['nfs', 'samba', 'ftp']) {
        if (!cfg[k]?.length) cfg[k] = [{ ...empty[k] }]
      }
    } catch (e) {
      err = e.message
    }
  }

  function addRow() {
    cfg[tab] = [...cfg[tab], { ...empty[tab] }]
  }

  function removeRow(i) {
    cfg[tab] = cfg[tab].filter((_, j) => j !== i)
    if (!cfg[tab].length) cfg[tab] = [{ ...empty[tab] }]
  }

  async function save() {
    msg = ''
    err = ''
    try {
      cfg = await api.sharesPut(tab, cfg[tab])
      msg = 'Partages enregistrés et services rechargés'
    } catch (e) {
      err = e.message
    }
  }

  async function reapply() {
    applying = true
    msg = ''
    err = ''
    try {
      const res = await api.sharesApply()
      msg = `NFS: ${res.nfs} · Samba: ${res.samba} · FTP: ${res.ftp}`
    } catch (e) {
      err = e.message
    } finally {
      applying = false
    }
  }
</script>

<p class="hint">Chemins sous <code>/volumes/nom</code> (montés depuis l'hôte via <strong>Montages</strong>). NFS : ACL par IP.</p>

<div class="tabs">
  {#each ['nfs', 'samba', 'ftp'] as t}
    <button class:active={tab === t} onclick={() => (tab = t)}>{t.toUpperCase()}</button>
  {/each}
  <button class="ghost" onclick={reapply} disabled={applying}>Recharger services</button>
</div>

{#if tab === 'nfs'}
  {#each cfg.nfs as row, i}
    <div class="form-row">
      <input placeholder="Chemin (/volumes/raid1/public)" bind:value={row.path} />
      <input placeholder="Clients" bind:value={row.clients} />
      <input placeholder="Options" bind:value={row.options} />
      <label class="chk"><input type="checkbox" bind:checked={row.enabled} /> Actif</label>
      <button class="ghost" onclick={() => removeRow(i)}>×</button>
    </div>
  {/each}
{:else if tab === 'samba'}
  {#each cfg.samba as row, i}
    <div class="form-row">
      <input placeholder="Nom" bind:value={row.name} />
      <input placeholder="Chemin (/volumes/…)" bind:value={row.path} />
      <label class="chk"><input type="checkbox" bind:checked={row.browseable} /> Navigable</label>
      <label class="chk"><input type="checkbox" bind:checked={row.read_only} /> Lecture seule</label>
      <label class="chk"><input type="checkbox" bind:checked={row.guest_ok} /> Invité</label>
      <label class="chk"><input type="checkbox" bind:checked={row.enabled} /> Actif</label>
      <button class="ghost" onclick={() => removeRow(i)}>×</button>
    </div>
  {/each}
{:else}
  {#each cfg.ftp as row, i}
    <div class="form-row">
      <input placeholder="Utilisateur FTP" bind:value={row.name} />
      <input placeholder="Racine (/volumes/…)" bind:value={row.path} />
      <label class="chk"><input type="checkbox" bind:checked={row.enabled} /> Actif</label>
      <button class="ghost" onclick={() => removeRow(i)}>×</button>
    </div>
  {/each}
{/if}

<div class="actions">
  <button class="ghost" onclick={addRow}>+ Ligne</button>
  <button onclick={save}>Enregistrer</button>
</div>
{#if msg}<p class="ok">{msg}</p>{/if}
{#if err}<p class="err">{err}</p>{/if}

<style>
  .tabs { display: flex; gap: 6px; margin-bottom: 12px; flex-wrap: wrap; }
  .tabs button { background: var(--bb-panel); font-size: 12px; padding: 6px 10px; }
  .tabs button.active { background: var(--bb-accent); }
  .form-row {
    display: grid;
    grid-template-columns: 1fr 1fr auto auto auto;
    gap: 6px;
    margin-bottom: 8px;
    align-items: center;
  }
  .chk { flex-direction: row; font-size: 11px; color: var(--bb-muted); white-space: nowrap; }
  .chk input { width: auto; }
  .actions { display: flex; gap: 8px; margin-top: 12px; }
  .ok { color: var(--bb-ok); margin-top: 8px; font-size: 12px; }
  .err { color: var(--bb-danger); margin-top: 8px; }
</style>
