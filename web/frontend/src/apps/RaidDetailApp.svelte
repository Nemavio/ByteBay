<script>
  import { api } from '../lib/api.js'
  import { poll, POLL_DETAIL_MS } from '../lib/poll.js'
  import ConfirmModal from '../lib/ConfirmModal.svelte'

  let { name = '' } = $props()

  let detail = $state(null)
  let loading = $state(true)
  let error = $state('')
  let actionMsg = $state('')
  let actionErr = $state('')
  let syncing = $state(false)

  let confirm = $state({ open: false, title: '', message: '', variant: 'warn', confirmLabel: 'Lancer', onOk: null })

  function openConfirm({ title, message, variant = 'warn', confirmLabel = 'Lancer', onOk }) {
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

  async function refresh({ silent = false } = {}) {
    if (!name) return
    if (!silent) {
      loading = true
      error = ''
    }
    try {
      detail = await api.raidDetail(name)
    } catch (e) {
      if (!silent) error = e.message
    } finally {
      if (!silent) loading = false
    }
  }

  $effect(() => {
    if (!name) return
    detail = null
    refresh()
    const stop = poll(() => refresh({ silent: true }), POLL_DETAIL_MS)
    return stop
  })

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

  let syncBusy = $derived(
    !!detail?.sync_action && detail.sync_action !== 'idle' && (detail.sync_percent ?? 0) < 100
  )

  function runSync(action) {
    if (!name || syncing) return
    if (action === 'idle') {
      executeSync(action)
      return
    }
    const prompts = {
      repair: {
        title: 'Réparation RAID',
        message: 'Lancer la réparation des secteurs incohérents ?',
        variant: 'danger',
        confirmLabel: 'Réparer',
      },
      check: {
        title: 'Vérification RAID',
        message: 'Lancer une vérification complète du RAID ? Cela peut prendre plusieurs heures.',
        variant: 'warn',
        confirmLabel: 'Vérifier',
      },
    }
    const p = prompts[action]
    if (!p) return
    openConfirm({ ...p, onOk: () => executeSync(action) })
  }

  async function executeSync(action) {
    syncing = true
    actionMsg = ''
    actionErr = ''
    try {
      detail = await api.raidSync(name, action)
      const labels = { check: 'Vérification', repair: 'Réparation', idle: 'Opération arrêtée' }
      actionMsg = labels[action] ? `${labels[action]} lancée.` : 'OK'
    } catch (e) {
      actionErr = e.message
    } finally {
      syncing = false
    }
  }
</script>

{#if loading && !detail}
  <p>Analyse du RAID…</p>
{:else if error && !detail}
  <p class="err">{error}</p>
{:else if detail}
  <div class="head">
    <span class="badge" class:ok={!detail.degraded} class:warn={detail.degraded}>
      {detail.degraded ? 'Dégradé' : 'Sain'}
    </span>
    <code>{detail.path}</code>
    <span class="muted">{raidLabel(detail.level)}</span>
    <span class="live" title="Mise à jour automatique">●</span>
  </div>

  {#if detail.degraded_reasons?.length}
    <section class="warn-box">
      <h4>Pourquoi dégradé ?</h4>
      <ul>
        {#each detail.degraded_reasons as r}
          <li>{r}</li>
        {/each}
      </ul>
    </section>
  {/if}

  <section class="maint">
    <h4>Maintenance</h4>
    <p class="hint">Vérification (check) : relit le volume et contrôle la parité. Réparation (repair) : corrige après un check ayant trouvé des erreurs.</p>
    <div class="maint-actions">
      <button onclick={() => runSync('check')} disabled={syncing || syncBusy}>Vérifier</button>
      <button onclick={() => runSync('repair')} disabled={syncing || syncBusy}>Réparer</button>
      {#if syncBusy}
        <button class="ghost" onclick={() => runSync('idle')} disabled={syncing}>Arrêter</button>
      {/if}
    </div>
    {#if actionMsg}<p class="ok">{actionMsg}</p>{/if}
    {#if actionErr}<p class="err">{actionErr}</p>{/if}
  </section>

  <div class="grid">
    <div><span class="lbl">État mdadm</span>{detail.md_state || detail.state || '—'}</div>
    <div><span class="lbl">Taille</span>{fmt(detail.size_bytes)}</div>
    <div><span class="lbl">Disques actifs</span>{detail.active_devices}/{detail.raid_devices}</div>
    <div><span class="lbl">En service</span>{detail.working_devices}</div>
    <div><span class="lbl">En échec</span>{detail.failed_devices}</div>
    <div><span class="lbl">Spare</span>{detail.spare_devices}</div>
    {#if detail.slot_map}
      <div><span class="lbl">Carte mdstat</span><code>[{detail.slot_map}]</code></div>
    {/if}
    {#if detail.sync_action}
      <div class="wide">
        <span class="lbl">{detail.sync_action}</span>
        <div class="sync-track">
          <div class="sync-fill" style="width:{detail.sync_percent || 0}%"></div>
        </div>
        {detail.sync_percent?.toFixed(1) ?? 0}%
      </div>
    {/if}
    {#if detail.rebuild_status}
      <div><span class="lbl">Rebuild</span>{detail.rebuild_status}</div>
    {/if}
    {#if detail.uuid}
      <div class="wide"><span class="lbl">UUID</span><code class="small">{detail.uuid}</code></div>
    {/if}
  </div>

  <h4>Emplacements</h4>
  <table>
    <thead>
      <tr><th>Slot</th><th>Disque</th><th>État</th></tr>
    </thead>
    <tbody>
      {#each detail.members?.length ? detail.members : [] as m}
        <tr class:bad={m.state === 'removed' || m.state.includes('faulty')}>
          <td>{m.slot}</td>
          <td><code>{m.device || '— manquant —'}</code></td>
          <td>{m.state}</td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

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
  .head { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; flex-wrap: wrap; }
  .badge { font-size: 11px; padding: 2px 8px; border-radius: 4px; background: var(--bb-border); }
  .badge.ok { background: rgba(62,207,142,0.2); color: var(--bb-ok); }
  .badge.warn { background: rgba(231,76,92,0.15); color: var(--bb-danger); }
  .muted { color: var(--bb-muted); font-size: 12px; }
  .live { color: var(--bb-ok); font-size: 10px; margin-left: auto; animation: pulse 2s ease-in-out infinite; }
  @keyframes pulse { 0%, 100% { opacity: 0.35; } 50% { opacity: 1; } }
  .warn-box {
    margin-bottom: 12px;
    padding: 10px;
    border-radius: 6px;
    border: 1px solid rgba(231,76,92,0.35);
    background: rgba(231,76,92,0.08);
  }
  .warn-box h4 { font-size: 12px; margin: 0 0 6px; color: var(--bb-danger); }
  .warn-box ul { margin: 0; padding-left: 18px; font-size: 12px; }
  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px 16px;
    margin-bottom: 14px;
    font-size: 12px;
  }
  .wide { grid-column: 1 / -1; }
  .lbl { display: block; font-size: 10px; color: var(--bb-muted); text-transform: uppercase; letter-spacing: 0.04em; }
  h4 { font-size: 12px; color: var(--bb-muted); margin-bottom: 6px; }
  code { font-size: 11px; }
  code.small { word-break: break-all; }
  .sync-track { height: 6px; background: var(--bb-border); border-radius: 3px; margin: 4px 0; overflow: hidden; }
  .sync-fill { height: 100%; background: var(--bb-accent, #4a9eff); transition: width 0.3s; }
  tr.bad td { color: var(--bb-danger); }
  .err { color: var(--bb-danger); }
  .ok { color: var(--bb-ok); font-size: 12px; margin-top: 6px; }
  .maint { margin-bottom: 14px; }
  .maint h4 { font-size: 12px; color: var(--bb-muted); margin-bottom: 4px; }
  .hint { font-size: 11px; color: var(--bb-muted); margin: 0 0 8px; line-height: 1.4; }
  .maint-actions { display: flex; flex-wrap: wrap; gap: 8px; }
  .maint-actions button { font-size: 12px; padding: 5px 12px; }
  .maint-actions .ghost { background: transparent; border: 1px solid var(--bb-border); }
</style>
