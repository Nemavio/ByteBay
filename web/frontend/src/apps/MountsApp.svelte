<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let mounts = $state([])
  let arrays = $state([])
  let volumes = $state([])
  let loading = $state(true)
  let error = $state('')
  let msg = $state('')

  let name = $state('')
  let source = $state('')
  let fstype = $state('ext4')
  let formatDisk = $state(false)
  let creating = $state(false)
  let activeJob = $state(null)
  let pollTimer = null

  onMount(() => { load() })

  async function load() {
    loading = true
    error = ''
    const [m, r, v] = await Promise.allSettled([
      api.mounts(),
      api.raid(),
      api.volumes(),
    ])
    mounts = m.status === 'fulfilled' ? m.value : []
    if (m.status === 'rejected') {
      error = m.reason?.message || 'Impossible de charger les montages'
    }
    arrays = r.status === 'fulfilled' ? r.value : []
    volumes = v.status === 'fulfilled' ? v.value : []
    if (!source && arrays.length) source = arrays[0].path
    loading = false
  }

  function stopPolling() {
    if (pollTimer) {
      clearTimeout(pollTimer)
      pollTimer = null
    }
  }

  async function pollJob(id) {
    try {
      const job = await api.mountJob(id)
      activeJob = job
      if (job.status === 'done') {
        creating = false
        stopPolling()
        msg = job.mount
          ? `${job.mount.name} monté → ${job.mount.container_path}`
          : 'Volume prêt'
        name = ''
        formatDisk = false
        activeJob = null
        await load()
        return
      }
      if (job.status === 'error') {
        creating = false
        stopPolling()
        error = job.error || 'Échec du formatage'
        activeJob = null
        return
      }
      pollTimer = setTimeout(() => pollJob(id), 600)
    } catch (e) {
      creating = false
      stopPolling()
      error = e.message
      activeJob = null
    }
  }

  async function create() {
    if (!name.trim() || !source) {
      error = 'Nom et source requis'
      return
    }
    if (formatDisk && !confirm(`Formater ${source} en ${fstype} ? Toutes les données seront effacées.`)) {
      return
    }
    creating = true
    error = ''
    msg = ''
    stopPolling()
    activeJob = null
    try {
      const res = await api.mountCreate({
        name: name.trim(),
        source,
        fstype,
        format: formatDisk,
        options: 'defaults',
      })
      if (res.id) {
        activeJob = res
        pollJob(res.id)
        return
      }
      msg = `${res.name} monté → ${res.container_path}`
      name = ''
      formatDisk = false
      creating = false
      await load()
    } catch (e) {
      error = e.message
      creating = false
    }
  }

  async function remove(mp) {
    if (!confirm(`Démonter et retirer ${mp.name} ?`)) return
    try {
      await api.mountDelete(mp.name)
      msg = `${mp.name} démonté`
      await load()
    } catch (e) {
      error = e.message
    }
  }

  function phaseLabel(status) {
    if (status === 'mounting') return 'Montage'
    if (status === 'formatting') return 'Formatage'
    return status
  }
</script>

<p class="hint">
  Les volumes sont montés sur l'hôte sous <code>/srv/bytebay-volumes</code> puis exposés
  dynamiquement dans l'engine sous <code>/volumes/…</code> (sans redémarrer Docker).
  Le formatage s'exécute en arrière-plan avec suivi de progression.
</p>

{#if activeJob}
  <section class="progress-box">
    <div class="progress-head">
      <strong>{phaseLabel(activeJob.status)}</strong>
      <span>{activeJob.progress}%</span>
    </div>
    <div class="progress-track">
      <div class="progress-fill" style="width:{activeJob.progress}%"></div>
    </div>
    <p class="progress-msg">{activeJob.message}</p>
  </section>
{/if}

<section class="create">
  <h3>Nouveau point de montage</h3>
  <div class="row2">
    <label>Nom (dossier)
      <input placeholder="raid1" bind:value={name} disabled={creating} />
    </label>
    <label>Source
      <select bind:value={source} disabled={creating}>
        <option value="">—</option>
        {#each arrays as a}
          <option value={a.path}>{a.path}</option>
        {/each}
      </select>
    </label>
  </div>
  <div class="row2">
    <label>Système de fichiers
      <select bind:value={fstype} disabled={creating}>
        <option value="ext4">ext4</option>
        <option value="xfs">xfs</option>
        <option value="btrfs">btrfs</option>
      </select>
    </label>
    <label class="chk-row">
      <input type="checkbox" bind:checked={formatDisk} disabled={creating} />
      Formater avant montage
    </label>
  </div>
  <button onclick={create} disabled={creating || !name.trim() || !source}>
    {creating ? (formatDisk ? 'Formatage en cours…' : 'Montage…') : 'Monter le volume'}
  </button>
</section>

<hr />

<h3>Volumes configurés</h3>
{#if loading}
  <p>Chargement…</p>
{:else if mounts.length === 0}
  <p class="muted">Aucun volume. Montez un array RAID ou disque pour commencer.</p>
{:else}
  <table>
    <thead>
      <tr>
        <th>Nom</th><th>Source</th><th>Hôte</th><th>Engine</th><th>État</th><th></th>
      </tr>
    </thead>
    <tbody>
      {#each mounts as m}
        <tr>
          <td><strong>{m.name}</strong></td>
          <td><code>{m.source}</code></td>
          <td class="small"><code>{m.host_path}</code></td>
          <td><code>{m.container_path}</code></td>
          <td>{m.mounted ? '✓ monté' : '— démonté'}{#if volumes.some((v) => v.path === m.container_path)} · visible engine{/if}</td>
          <td><button class="danger" onclick={() => remove(m)} disabled={creating}>Démonter</button></td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

{#if msg}<p class="ok">{msg}</p>{/if}
{#if error}<p class="err">{error}</p>{/if}

<style>
  h3 { font-size: 13px; margin-bottom: 8px; color: var(--bb-muted); font-weight: 600; }
  .hint, .muted { color: var(--bb-muted); font-size: 11px; margin-bottom: 10px; }
  .create { margin-bottom: 14px; }
  .row2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 8px; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--bb-muted); }
  .chk-row { flex-direction: row; align-items: center; gap: 8px; color: var(--bb-text); padding-top: 18px; }
  .chk-row input { width: auto; }
  hr { border: none; border-top: 1px solid var(--bb-border); margin: 14px 0; }
  .small { font-size: 10px; max-width: 120px; word-break: break-all; }
  code { font-size: 11px; }
  .ok { color: var(--bb-ok); margin-top: 8px; }
  .err { color: var(--bb-danger); margin-top: 8px; }
  .progress-box {
    margin-bottom: 14px;
    padding: 12px;
    border: 1px solid var(--bb-border);
    border-radius: 8px;
    background: var(--bb-panel);
  }
  .progress-head {
    display: flex;
    justify-content: space-between;
    font-size: 12px;
    margin-bottom: 8px;
  }
  .progress-track {
    height: 8px;
    background: var(--bb-border);
    border-radius: 4px;
    overflow: hidden;
  }
  .progress-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--bb-accent, #4a9eff), var(--bb-ok, #3ecf8e));
    transition: width 0.4s ease;
  }
  .progress-msg {
    margin: 8px 0 0;
    font-size: 11px;
    color: var(--bb-muted);
  }
</style>
