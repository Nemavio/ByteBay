<script>
  import { onMount } from 'svelte'
  import { api } from '../../lib/api.js'
  import { useDesktop } from '../../lib/desktop.js'
  import { useWindowHost } from '../../lib/windowHost.js'
  import { saveMountJob } from '../../lib/jobStorage.js'
  import ConfirmModal from '../../lib/ConfirmModal.svelte'

  let { onSuccess = () => {}, onJobStarted = () => {} } = $props()

  const desktop = useDesktop()
  const host = useWindowHost()

  let arrays = $state([])
  let metaLoading = $state(true)
  let name = $state('')
  let source = $state('')
  let fstype = $state('ext4')
  let formatDisk = $state(false)
  let err = $state('')
  let busy = $state(false)
  let confirmOpen = $state(false)

  onMount(async () => {
    try {
      arrays = await api.raid()
      if (arrays.length) source = raidSource(arrays[0])
    } catch (e) {
      err = e.message
    } finally {
      metaLoading = false
    }
  })

  function raidSource(a) {
    return a.stable_path || a.path
  }

  function closeSelf() {
    if (host?.key) desktop?.closeWindow?.(host.key)
  }

  async function doCreate() {
    busy = true
    err = ''
    try {
      const res = await api.mountCreate({
        name: name.trim(),
        source,
        fstype,
        format: formatDisk,
        options: 'defaults',
      })
      if (res.id) {
        saveMountJob(res.id)
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

  function submit(e) {
    e.preventDefault()
    if (!name.trim() || !source) {
      err = 'Nom et source requis'
      return
    }
    if (formatDisk) {
      confirmOpen = true
      return
    }
    doCreate()
  }
</script>

<form class="form" onsubmit={submit}>
  <div class="row2">
    <label>Nom (dossier)
      <input placeholder="raid1" bind:value={name} disabled={busy} />
    </label>
    <label>Source
      <select bind:value={source} disabled={busy || metaLoading}>
        <option value="">{metaLoading ? 'Chargement…' : '—'}</option>
        {#each arrays as a}
          <option value={raidSource(a)}>{raidSource(a)}</option>
        {/each}
      </select>
    </label>
  </div>
  <div class="row2">
    <label>Système de fichiers
      <select bind:value={fstype} disabled={busy}>
        <option value="ext4">ext4</option>
        <option value="xfs">xfs</option>
        <option value="btrfs">btrfs</option>
      </select>
    </label>
    <label class="chk-row">
      <input type="checkbox" bind:checked={formatDisk} disabled={busy} />
      Formater avant montage
    </label>
  </div>
  {#if err}<p class="err">{err}</p>{/if}
  <div class="actions">
    <button type="button" class="ghost" onclick={closeSelf} disabled={busy}>Annuler</button>
    <button type="submit" disabled={busy || !name.trim() || !source}>
      {busy ? (formatDisk ? 'Formatage…' : 'Montage…') : 'Monter le volume'}
    </button>
  </div>
</form>

<ConfirmModal
  open={confirmOpen}
  title="Formater le disque"
  message={source ? `Formater ${source} en ${fstype} ? Toutes les données seront effacées.` : ''}
  variant="danger"
  confirmLabel="Formater"
  onconfirm={() => { confirmOpen = false; doCreate() }}
  oncancel={() => (confirmOpen = false)}
/>

<style>
  .form { display: flex; flex-direction: column; gap: 8px; }
  .row2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--bb-muted); }
  .chk-row { flex-direction: row; align-items: center; gap: 8px; color: var(--bb-text); padding-top: 18px; }
  .chk-row input { width: auto; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; }
  .err { color: var(--bb-danger); font-size: 12px; margin: 0; }
</style>
