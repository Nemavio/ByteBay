<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { poll, POLL_LIST_MS } from '../lib/poll.js'
  import { saveMountJob, clearMountJob, loadMountJob } from '../lib/jobStorage.js'
  import ConfirmModal from '../lib/ConfirmModal.svelte'
  import { useDesktop } from '../lib/desktop.js'
  import { useWindowHost } from '../lib/windowHost.js'
  import MountCreateForm from './forms/MountCreateForm.svelte'

  const desktop = useDesktop()
  const host = useWindowHost()

  let mounts = $state([])
  let volumes = $state([])
  let mountsReady = $state(false)
  let error = $state('')
  let msg = $state('')

  let creating = $state(false)
  let activeJob = $state(null)
  let pollTimer = null

  let confirm = $state({ open: false, title: '', message: '', variant: 'warn', confirmLabel: 'Confirmer', onOk: null })

  function openConfirm({ title, message, variant = 'warn', confirmLabel = 'Confirmer', onOk }) {
    confirm = { open: true, title, message, variant, confirmLabel, onOk }
  }

  function closeConfirm() {
    confirm = { ...confirm, open: false, onOk: null }
  }

  function handleConfirm() {
    const fn = confirm.onOk
    closeConfirm()
    fn?.()
  }

  onMount(() => {
    load()
    resumePendingJob()
    const stop = poll(() => load({ silent: true }), POLL_LIST_MS)
    return stop
  })

  async function resumePendingJob() {
    try {
      const hk = await api.housekeeping()
      const fromHk = (hk.items || []).find((i) => i.kind === 'mount_job' && i.id)
      const id = fromHk?.id || loadMountJob()
      if (!id) return
      creating = true
      saveMountJob(id)
      pollJob(id)
    } catch {
      const id = loadMountJob()
      if (id) {
        creating = true
        pollJob(id)
      }
    }
  }

  async function load() {
    error = ''
    mountsReady = false
    try {
      mounts = await api.mounts()
      if (!Array.isArray(mounts)) mounts = []
    } catch (e) {
      mounts = []
      error = e.message || 'Impossible de charger les montages'
    } finally {
      mountsReady = true
    }
    try {
      volumes = await api.volumes()
    } catch {
      volumes = []
    }
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
        clearMountJob()
        msg = job.mount
          ? `${job.mount.name} monté → ${job.mount.container_path}`
          : 'Volume prêt'
        activeJob = null
        await load()
        return
      }
      if (job.status === 'error') {
        creating = false
        stopPolling()
        clearMountJob()
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
      clearMountJob()
    }
  }

  function openCreateForm() {
    error = ''
    msg = ''
    desktop?.openCustomWindow({
      title: 'Nouveau point de montage',
      component: MountCreateForm,
      props: {
        onSuccess: () => load(),
        onJobStarted: (id) => {
          creating = true
          saveMountJob(id)
          pollJob(id)
        },
      },
      w: 520,
      h: 320,
      from: host,
    })
  }

  function remove(mp) {
    openConfirm({
      title: 'Démonter le volume',
      message: `Démonter et retirer ${mp.name} ?`,
      variant: 'danger',
      confirmLabel: 'Démonter',
      onOk: () => doRemove(mp),
    })
  }

  async function doRemove(mp) {
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
  <div class="section-head">
    <h3>Points de montage</h3>
    <button type="button" onclick={openCreateForm} disabled={creating}>+ Ajouter un point de montage</button>
  </div>
</section>

<hr />

<h3>Volumes configurés</h3>
{#if !mountsReady}
  <p>Chargement…</p>
{:else if mounts.length === 0}
  <p class="muted">Aucun montage actif.</p>
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

<ConfirmModal
  open={confirm.open}
  title={confirm.title}
  message={confirm.message}
  variant={confirm.variant}
  confirmLabel={confirm.confirmLabel}
  onconfirm={handleConfirm}
  oncancel={closeConfirm}
/>

<style>
  h3 { font-size: 13px; margin-bottom: 8px; color: var(--bb-muted); font-weight: 600; }
  .hint, .muted { color: var(--bb-muted); font-size: 11px; margin-bottom: 10px; }
  .create { margin-bottom: 14px; }
  .section-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 8px; }
  .section-head h3 { margin-bottom: 0; }
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
