<script>
  import { api } from '../lib/api.js'
  import { useDesktop } from '../lib/desktop.js'
  import { useWindowHost } from '../lib/windowHost.js'
  import { poll, POLL_LIST_MS } from '../lib/poll.js'
  import SmartDetailApp from './SmartDetailApp.svelte'

  const desktop = useDesktop()
  const host = useWindowHost()

  let disks = $state([])
  let alerts = $state([])
  let lastScan = $state('')
  let loading = $state(true)
  let error = $state('')

  async function refresh({ silent = false } = {}) {
    if (!silent) loading = true
    if (!silent) error = ''
    try {
      const [scan, al] = await Promise.all([api.smartAll(), api.smartAlerts()])
      disks = (scan.disks || []).filter((d) => d.available)
      lastScan = scan.last_scan || ''
      alerts = al.alerts || []
    } catch (e) {
      if (!silent) error = e.message
    } finally {
      if (!silent) loading = false
    }
  }

  $effect(() => {
    refresh()
    const stop = poll(() => refresh({ silent: true }), POLL_LIST_MS)
    return stop
  })

  function openDetail(name, device) {
    desktop.openCustomWindow({
      title: `SMART — ${device}`,
      component: SmartDetailApp,
      props: { device: name },
      w: 520,
      h: 480,
      from: host,
    })
  }
</script>

<div class="toolbar">
  <button onclick={refresh} disabled={loading}>{loading ? 'Scan…' : 'Scanner maintenant'}</button>
  {#if lastScan}<span class="muted">Dernier scan : {new Date(lastScan).toLocaleString('fr-FR')}</span>{/if}
</div>

{#if alerts.length}
  <div class="alerts">
    <h3>Alertes ({alerts.length})</h3>
    {#each alerts.slice(0, 8) as a}
      <div class="alert" class:critical={a.severity === 'critical'}>
        <strong>{a.device}</strong> — {a.message}
      </div>
    {/each}
  </div>
{/if}

{#if error}
  <p class="err">{error}</p>
{:else if !disks.length && !loading}
  <p class="muted empty">Aucun disque avec données SMART.</p>
{:else}
  <table>
    <thead>
      <tr><th>Disque</th><th>Modèle</th><th>État</th><th>Temp.</th><th></th></tr>
    </thead>
    <tbody>
      {#each disks as d}
        <tr>
          <td><code>{d.device}</code></td>
          <td>{d.model || '—'}</td>
          <td>
            <span class="badge" class:ok={d.healthy} class:warn={!d.healthy}>
              {d.healthy ? 'Sain' : 'Dégradé'}
            </span>
          </td>
          <td>{d.temp_c != null ? `${d.temp_c} °C` : '—'}</td>
          <td><button class="ghost" onclick={(e) => { e.stopPropagation(); openDetail(d.name, d.device) }}>Détails</button></td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

<style>
  .toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
  .muted { color: var(--bb-muted); font-size: 11px; }
  .alerts { background: rgba(231,76,92,0.08); border: 1px solid var(--bb-border); border-radius: 8px; padding: 10px; margin-bottom: 12px; }
  .alerts h3 { font-size: 12px; margin-bottom: 8px; color: var(--bb-danger); }
  .alert { font-size: 12px; margin-bottom: 4px; }
  .err { color: var(--bb-danger); }
  .empty { margin-top: 8px; font-size: 12px; }
  code { font-size: 11px; }
</style>
