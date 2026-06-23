<script>
  import { onMount } from 'svelte'
  import { api } from '../../lib/api.js'
  import { useDesktop } from '../../lib/desktop.js'
  import { useWindowHost } from '../../lib/windowHost.js'
  import { saveRaidJob } from '../../lib/jobStorage.js'

  let { onSuccess = () => {}, onJobStarted = () => {} } = $props()

  const desktop = useDesktop()
  const host = useWindowHost()

  let disks = $state([])
  let disksLoading = $state(true)
  let level = $state('6')
  let raidDevices = $state(4)
  let selected = $state([])
  let err = $state('')
  let busy = $state(false)

  onMount(async () => {
    try {
      const d = await api.disks()
      disks = (Array.isArray(d) ? d : []).filter((x) => !x.in_raid && !x.mountpoint && !x.name.startsWith('md'))
    } catch (e) {
      err = e.message
    } finally {
      disksLoading = false
    }
  })

  function closeSelf() {
    if (host?.key) desktop?.closeWindow?.(host.key)
  }

  function toggle(dev) {
    selected = selected.includes(dev) ? selected.filter((d) => d !== dev) : [...selected, dev]
  }

  function fmt(bytes) {
    if (!bytes) return '—'
    const u = ['o', 'Ko', 'Mo', 'Go', 'To']
    let i = 0, n = bytes
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
    return `${n.toFixed(1)} ${u[i]}`
  }

  async function submit(e) {
    e.preventDefault()
    if (selected.length < 1) {
      err = 'Sélectionnez au moins 1 disque'
      return
    }
    if (raidDevices < selected.length) {
      err = 'raid_devices doit être ≥ disques sélectionnés'
      return
    }
    busy = true
    err = ''
    try {
      const res = await api.raidCreate({ level, devices: selected, raid_devices: raidDevices })
      if (res.id) {
        saveRaidJob(res.id)
        onJobStarted(res.id)
        onSuccess()
        closeSelf()
        return
      }
      onSuccess()
      closeSelf()
    } catch (e) {
      err = e.message
    } finally {
      busy = false
    }
  }
</script>

<form class="form" onsubmit={submit}>
  <div class="row2">
    <label>Niveau
      <select bind:value={level} disabled={busy}>
        <option value="0">RAID 0</option>
        <option value="1">RAID 1</option>
        <option value="5">RAID 5</option>
        <option value="6">RAID 6</option>
        <option value="10">RAID 10</option>
      </select>
    </label>
    <label>Emplacements totaux
      <input type="number" min="2" max="32" bind:value={raidDevices} disabled={busy} />
    </label>
  </div>
  <p class="hint">
    RAID 6 à 4 disques avec 3 présents : sélectionnez 3 disques, mettez 4 emplacements.
  </p>
  <div class="disk-pick">
    {#if disksLoading}
      <p class="muted">Chargement des disques…</p>
    {:else if disks.length === 0}
      <p class="muted">Aucun disque libre.</p>
    {:else}
      {#each disks as d}
        <label class="pick">
          <input type="checkbox" checked={selected.includes(d.path)} onchange={() => toggle(d.path)} disabled={busy} />
          <span>{d.path}</span>
          <span class="muted">{fmt(d.size_bytes)}</span>
        </label>
      {/each}
    {/if}
  </div>
  {#if err}<p class="err">{err}</p>{/if}
  <div class="actions">
    <button type="button" class="ghost" onclick={closeSelf} disabled={busy}>Annuler</button>
    <button type="submit" disabled={busy || selected.length < 1}>{busy ? 'Création…' : 'Créer le RAID'}</button>
  </div>
</form>

<style>
  .form { display: flex; flex-direction: column; gap: 8px; }
  .row2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--bb-muted); }
  .hint, .muted { color: var(--bb-muted); font-size: 11px; margin: 0; }
  .disk-pick { max-height: 140px; overflow: auto; }
  .pick { flex-direction: row; align-items: center; gap: 8px; display: flex; color: var(--bb-text); }
  .pick input { width: auto; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; }
  .err { color: var(--bb-danger); font-size: 12px; margin: 0; }
</style>
