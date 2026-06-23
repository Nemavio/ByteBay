<script>
  import { api } from '../lib/api.js'
  import { poll, POLL_DETAIL_MS } from '../lib/poll.js'

  let { device = '' } = $props()

  let info = $state(null)
  let loading = $state(true)
  let error = $state('')

  async function refresh({ silent = false } = {}) {
    if (!device) return
    if (!silent) {
      loading = true
      error = ''
    }
    try {
      info = await api.smart(device)
    } catch (e) {
      if (!silent) error = e.message
    } finally {
      if (!silent) loading = false
    }
  }

  $effect(() => {
    if (!device) return
    info = null
    refresh()
    const stop = poll(() => refresh({ silent: true }), POLL_DETAIL_MS)
    return stop
  })
</script>

{#if loading && !info}
  <p>Analyse SMART…</p>
{:else if error && !info}
  <p class="err">{error}</p>
{:else if info}
  <div class="head">
    <span class="badge" class:ok={info.healthy} class:warn={!info.healthy}>
      {info.healthy ? 'Sain' : 'Attention'}
    </span>
    <code>{info.device}</code>
    <span class="live" title="Mise à jour automatique">●</span>
  </div>
  <div class="grid">
    <div><span class="lbl">Modèle</span>{info.model || '—'}</div>
    <div><span class="lbl">Série</span>{info.serial || '—'}</div>
    <div><span class="lbl">Température</span>{info.temp_c != null ? `${info.temp_c} °C` : '—'}</div>
    <div><span class="lbl">Heures</span>{info.power_on_hours ?? '—'}</div>
  </div>
  {#if info.attributes}
    <table>
      <thead><tr><th>Attribut</th><th>Valeur</th></tr></thead>
      <tbody>
        {#each Object.entries(info.attributes) as [k, v]}
          <tr><td>{k}</td><td>{v}</td></tr>
        {/each}
      </tbody>
    </table>
  {/if}
{/if}

<style>
  .head { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
  code { font-size: 12px; }
  .badge { font-size: 11px; padding: 2px 8px; border-radius: 4px; background: var(--bb-border); }
  .badge.ok { background: rgba(62,207,142,0.2); color: var(--bb-ok); }
  .badge.warn { background: rgba(231,76,92,0.15); color: var(--bb-danger); }
  .live { color: var(--bb-ok); font-size: 10px; margin-left: auto; animation: pulse 2s ease-in-out infinite; }
  @keyframes pulse { 0%, 100% { opacity: 0.35; } 50% { opacity: 1; } }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 12px; }
  .lbl { display: block; font-size: 10px; color: var(--bb-muted); margin-bottom: 2px; }
  table { font-size: 11px; }
  .err { color: var(--bb-danger); }
</style>
