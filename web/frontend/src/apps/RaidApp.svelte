<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { poll, POLL_LIST_MS } from '../lib/poll.js'
  import { saveRaidJob, clearRaidJob, loadRaidJob } from '../lib/jobStorage.js'
  import { useDesktop } from '../lib/desktop.js'
  import { useWindowHost } from '../lib/windowHost.js'
  import PasswordPrompt from '../lib/PasswordPrompt.svelte'
  import RaidDetailApp from './RaidDetailApp.svelte'
  import RaidCreateForm from './forms/RaidCreateForm.svelte'

  const desktop = useDesktop()
  const host = useWindowHost()

  let arrays = $state([])
  let disks = $state([])
  let arraysReady = $state(false)
  let disksLoading = $state(true)
  let error = $state('')
  let msg = $state('')
  let stopTarget = $state(null)

  let creating = $state(false)
  let activeJob = $state(null)
  let pollTimer = null
  let addDev = $state('')
  let addTarget = $state('')

  onMount(() => {
    load()
    resumePendingJob()
    const stop = poll(() => load({ silent: true }), POLL_LIST_MS)
    return stop
  })

  async function resumePendingJob() {
    try {
      const hk = await api.housekeeping()
      const fromHk = (hk.items || []).find((i) => i.kind === 'raid_job' && i.id)
      const id = fromHk?.id || loadRaidJob()
      if (!id) return
      creating = true
      saveRaidJob(id)
      pollJob(id)
    } catch {
      const id = loadRaidJob()
      if (id) {
        creating = true
        pollJob(id)
      }
    }
  }

  function openDetail(a) {
    desktop?.openCustomWindow({
      title: `RAID — ${a.path}`,
      component: RaidDetailApp,
      props: { name: a.name },
      w: 620,
      h: 520,
      from: host,
    })
  }

  async function load({ silent = false } = {}) {
    if (!silent) {
      error = ''
      arraysReady = false
      disksLoading = true
    }
    try {
      const r = await api.raid()
      arrays = Array.isArray(r) ? r : []
    } catch (e) {
      arrays = []
      if (!silent) error = e.message
    } finally {
      if (!silent) arraysReady = true
    }
    try {
      const d = await api.disks()
      disks = (Array.isArray(d) ? d : []).filter((x) => !x.in_raid && !x.mountpoint && !x.name.startsWith('md'))
    } catch (e) {
      if (!silent && !error) error = e.message
      disks = []
    } finally {
      if (!silent) disksLoading = false
    }
  }

  function openCreateForm() {
    desktop?.openCustomWindow({
      title: 'Créer un array RAID',
      component: RaidCreateForm,
      props: {
        onSuccess: () => load(),
        onJobStarted: (id) => {
          creating = true
          saveRaidJob(id)
          pollJob(id)
        },
      },
      w: 520,
      h: 460,
      from: host,
    })
  }

  function stopPolling() {
    if (pollTimer) {
      clearTimeout(pollTimer)
      pollTimer = null
    }
  }

  function phaseLabel(status) {
    if (status === 'preparing') return 'Préparation'
    if (status === 'creating') return 'Création'
    if (status === 'syncing') return 'Synchronisation'
    return status
  }

  async function pollJob(id) {
    try {
      const job = await api.raidJob(id)
      activeJob = job
      if (job.status === 'done') {
        creating = false
        stopPolling()
        clearRaidJob()
        await load()
        if (!arrays.length && job.array?.path) {
          error = `${job.array.path} signalé prêt mais absent de la liste — vérifiez mdadm sur l'hôte`
          msg = ''
        } else {
          msg = job.message || (job.array?.degraded
            ? `RAID créé en mode dégradé (${job.array.path})`
            : `Array RAID créé (${job.array?.path || ''})`)
        }
        activeJob = null
        return
      }
      if (job.status === 'error') {
        creating = false
        stopPolling()
        clearRaidJob()
        error = job.error || 'Échec de la création RAID'
        activeJob = null
        return
      }
      pollTimer = setTimeout(() => pollJob(id), 800)
    } catch (e) {
      creating = false
      stopPolling()
      error = e.message
      activeJob = null
      clearRaidJob()
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

  function stop(name) {
    error = ''
    msg = ''
    stopTarget = name
  }

  async function confirmStop(password) {
    if (!stopTarget) return
    await api.raidStop(stopTarget, password)
    msg = `${stopTarget} arrêté`
    stopTarget = null
    await load()
  }

  function cancelStop() {
    stopTarget = null
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
    <h3>Arrays RAID</h3>
    <button type="button" onclick={openCreateForm} disabled={creating}>+ Ajouter un RAID</button>
  </div>
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

<h3>Arrays existants <button class="ghost small" onclick={load} disabled={!arraysReady && disksLoading}>Actualiser</button></h3>
{#if !arraysReady}
  <p>Chargement…</p>
{:else if arrays.length === 0}
  <p class="muted">Aucun array RAID.</p>
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
            <button class="ghost" onclick={(e) => { e.stopPropagation(); openDetail(a) }}>Détails</button>
            <button class="danger" onclick={() => stop(a.name)}>Arrêter</button>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

{#if msg}<p class="ok">{msg}</p>{/if}
{#if error}<p class="err">{error}</p>{/if}

<PasswordPrompt
  open={!!stopTarget}
  title="Arrêter le RAID"
  message={stopTarget ? `Confirmez l'arrêt de ${stopTarget}. Cette action nécessite votre mot de passe.` : ''}
  confirmLabel="Arrêter le RAID"
  onconfirm={confirmStop}
  oncancel={cancelStop}
/>

<style>
  h3 { font-size: 13px; margin-bottom: 8px; color: var(--bb-muted); font-weight: 600; }
  h3 .small { margin-left: 8px; padding: 2px 8px; font-size: 11px; vertical-align: middle; }
  .create, .add { margin-bottom: 14px; }
  .section-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 8px; }
  .section-head h3 { margin-bottom: 0; }
  .row2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 8px; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--bb-muted); }
  .hint, .muted { color: var(--bb-muted); font-size: 11px; margin-bottom: 8px; }
  hr { border: none; border-top: 1px solid var(--bb-border); margin: 14px 0; }
  .devs { font-size: 11px; max-width: 160px; word-break: break-all; }
  .actions { display: flex; gap: 6px; flex-wrap: wrap; }
  .ok { color: var(--bb-ok); margin-top: 8px; }
  .err { color: var(--bb-danger); margin-top: 8px; }
  code { font-size: 11px; }
  .progress-box {
    margin-bottom: 14px; padding: 12px; border-radius: 8px;
    border: 1px solid var(--bb-border); background: rgba(0,0,0,0.15);
  }
  .progress-head { display: flex; justify-content: space-between; margin-bottom: 8px; font-size: 12px; }
  .progress-track { height: 8px; background: var(--bb-border); border-radius: 4px; overflow: hidden; }
  .progress-fill { height: 100%; background: var(--bb-accent); transition: width 0.3s ease; }
  .progress-msg { margin-top: 8px; font-size: 11px; color: var(--bb-muted); }
</style>
