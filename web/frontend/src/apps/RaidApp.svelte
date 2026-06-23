<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { useDesktop } from '../lib/desktop.js'
  import RaidDetailApp from './RaidDetailApp.svelte'

  const desktop = useDesktop()

  let arrays = $state([])
  let disks = $state([])
  let loading = $state(true)
  let error = $state('')
  let msg = $state('')

  let level = $state('6')
  let raidDevices = $state(4)
  let selected = $state([])
  let creating = $state(false)
  let addDev = $state('')
  let addTarget = $state('')

  onMount(() => { load() })

  function openDetail(a) {
    desktop?.openCustomWindow({
      title: `RAID — ${a.path}`,
      component: RaidDetailApp,
      props: { name: a.name },
      w: 620,
      h: 520,
    })
  }

  async function load() {
    loading = true
    error = ''
    try {
      const [r, d] = await Promise.all([api.raid(), api.disks()])
      arrays = r
      disks = d.filter((x) => !x.in_raid && !x.mountpoint && !x.name.startsWith('md'))
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  function toggle(dev) {
    selected = selected.includes(dev)
      ? selected.filter((d) => d !== dev)
      : [...selected, dev]
  }

  async function create() {
    if (selected.length < 1) {
      error = 'Sélectionnez au moins 1 disque'
      return
    }
    if (raidDevices < selected.length) {
      error = 'raid_devices doit être ≥ disques sélectionnés'
      return
    }
    creating = true
    error = ''
    msg = ''
    try {
      const body = { level, devices: selected, raid_devices: raidDevices }
      await api.raidCreate(body)
      msg = raidDevices > selected.length
        ? `RAID créé en mode dégradé (${selected.length}/${raidDevices} disques)`
        : 'Array RAID créé'
      selected = []
      await load()
    } catch (e) {
      error = e.message
    } finally {
      creating = false
    }
  }

  async function addDisk() {
    if (!addTarget || !addDev) return
    try {
      await api.raidAdd(addTarget, addDev)
      msg = `Disque ${addDev} ajouté à ${addTarget}`
      addDev = ''
      await load()
    } catch (e) {
      error = e.message
    }
  }

  async function stop(name) {
    if (!confirm(`Arrêter ${name} ?`)) return
    try {
      await api.raidStop(name)
      msg = `${name} arrêté`
      await load()
    } catch (e) {
      error = e.message
    }
  }

  function fmt(bytes) {
    if (!bytes) return '—'
    const u = ['o', 'Ko', 'Mo', 'Go', 'To']
    let i = 0, n = bytes
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
    return `${n.toFixed(1)} ${u[i]}`
  }

  function raidLabel(level) {
    const n = String(level ?? '').replace(/^raid/i, '')
    return n ? `RAID ${n}` : '—'
  }
</script>

<section class="create">
  <h3>Créer un array</h3>
  <div class="row2">
    <label>Niveau
      <select bind:value={level}>
        <option value="0">RAID 0</option>
        <option value="1">RAID 1</option>
        <option value="5">RAID 5</option>
        <option value="6">RAID 6</option>
        <option value="10">RAID 10</option>
      </select>
    </label>
    <label>Emplacements totaux
      <input type="number" min="2" max="32" bind:value={raidDevices} />
    </label>
  </div>
  <p class="hint">
    RAID 6 à 4 disques avec 3 présents : sélectionnez 3 disques, mettez 4 emplacements.
    Le 4<sup>e</sup> sera ajouté plus tard.
  </p>
  <div class="disk-pick">
    {#each disks as d}
      <label class="pick">
        <input type="checkbox" checked={selected.includes(d.path)} onchange={() => toggle(d.path)} />
        <span>{d.path}</span>
        <span class="muted">{fmt(d.size_bytes)}</span>
      </label>
    {/each}
  </div>
  <button onclick={create} disabled={creating || selected.length < 1}>
    {creating ? 'Création…' : 'Créer le RAID'}
  </button>
</section>

<section class="add">
  <h3>Ajouter un disque à un array dégradé</h3>
  <div class="row2">
    <label>Array
      <select bind:value={addTarget}>
        <option value="">—</option>
        {#each arrays as a}
          <option value={a.name}>{a.path} ({raidLabel(a.level)})</option>
        {/each}
      </select>
    </label>
    <label>Disque
      <select bind:value={addDev}>
        <option value="">—</option>
        {#each disks as d}
          <option value={d.path}>{d.path}</option>
        {/each}
      </select>
    </label>
  </div>
  <button class="ghost" onclick={addDisk} disabled={!addTarget || !addDev}>Ajouter le disque</button>
</section>

<hr />

<h3>Arrays existants</h3>
{#if loading}
  <p>Chargement…</p>
{:else if arrays.length === 0}
  <p class="muted">Aucun array.</p>
{:else}
  <table>
    <thead>
      <tr><th>Array</th><th>Niveau</th><th>État</th><th>Taille</th><th>Disques</th><th></th></tr>
    </thead>
    <tbody>
      {#each arrays as a}
        <tr>
          <td><code>{a.path}</code></td>
          <td>{raidLabel(a.level)}</td>
          <td>{a.state}{#if a.degraded} ⚠ dégradé{/if}</td>
          <td>{fmt(a.size_bytes)}</td>
          <td class="devs">{(a.devices || []).join(', ')}</td>
          <td class="actions">
            <button class="ghost" onclick={() => openDetail(a)}>Détails</button>
            <button class="danger" onclick={() => stop(a.name)}>Arrêter</button>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

{#if msg}<p class="ok">{msg}</p>{/if}
{#if error}<p class="err">{error}</p>{/if}

<style>
  h3 { font-size: 13px; margin-bottom: 8px; color: var(--bb-muted); font-weight: 600; }
  .create, .add { margin-bottom: 14px; }
  .row2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 8px; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--bb-muted); }
  .hint, .muted { color: var(--bb-muted); font-size: 11px; margin-bottom: 8px; }
  .disk-pick { max-height: 100px; overflow: auto; margin-bottom: 10px; }
  .pick { flex-direction: row; align-items: center; gap: 8px; display: flex; color: var(--bb-text); }
  .pick input { width: auto; }
  hr { border: none; border-top: 1px solid var(--bb-border); margin: 14px 0; }
  .devs { font-size: 11px; max-width: 160px; word-break: break-all; }
  .actions { display: flex; gap: 6px; flex-wrap: wrap; }
  .ok { color: var(--bb-ok); margin-top: 8px; }
  .err { color: var(--bb-danger); margin-top: 8px; }
  code { font-size: 11px; }
</style>
