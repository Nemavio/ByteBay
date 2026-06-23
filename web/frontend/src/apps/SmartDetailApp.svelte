<script>
  import { api } from '../lib/api.js'

  let { device = '' } = $props()

  let info = $state(null)
  let loading = $state(true)
  let error = $state('')

  $effect(() => {
    if (!device) return
    loading = true
    error = ''
    info = null
    api.smart(device).then((d) => (info = d)).catch((e) => (error = e.message)).finally(() => (loading = false))
  })
</script>

{#if loading}
  <p>Analyse SMART…</p>
{:else if error}
  <p class="err">{error}</p>
{:else if info}
  <div class="head">
    <span class="badge" class:ok={info.healthy} class:warn={!info.healthy}>
      {info.healthy ? 'Sain' : 'Attention'}
    </span>
    <code>{info.device}</code>
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
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 12px; }
  .lbl { display: block; font-size: 10px; color: var(--bb-muted); margin-bottom: 2px; }
  table { font-size: 11px; }
  .err { color: var(--bb-danger); }
</style>
