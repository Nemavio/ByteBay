<script>
  import { api } from '../lib/api.js'

  let health = $state(null)

  $effect(() => {
    api.health().then((h) => (health = h)).catch(() => (health = { web: false, agent: false, engine: false }))
  })
</script>

<h2>Bienvenue sur ByteBay</h2>
<p class="sub">Gestionnaire NAS léger pour ARM</p>

<div class="cards">
  <div class="card">
    <span class="lbl">Panel web</span>
    <span class="badge ok">En ligne</span>
  </div>
  <div class="card">
    <span class="lbl">Agent hôte</span>
    {#if health}
      <span class="badge" class:ok={health.agent} class:warn={!health.agent}>
        {health.agent ? 'RAID / SMART' : 'Hors ligne'}
      </span>
    {:else}
      <span class="badge">…</span>
    {/if}
  </div>
  <div class="card">
    <span class="lbl">Engine partages</span>
    {#if health}
      <span class="badge" class:ok={health.engine} class:warn={!health.engine}>
        {health.engine ? 'NFS/Samba/FTP' : 'Hors ligne'}
      </span>
    {:else}
      <span class="badge">…</span>
    {/if}
  </div>
</div>

<p class="hint">RAID et SMART sur l'hôte · Partages réseau dans le conteneur engine.</p>

<style>
  h2 { font-size: 20px; margin-bottom: 4px; }
  .sub { color: var(--bb-muted); margin-bottom: 20px; }
  .cards { display: flex; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
  .card {
    flex: 1;
    min-width: 120px;
    background: var(--bb-panel);
    border: 1px solid var(--bb-border);
    border-radius: 8px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .lbl { font-size: 11px; color: var(--bb-muted); text-transform: uppercase; letter-spacing: 0.05em; }
  .hint { color: var(--bb-muted); font-size: 12px; }
</style>
