<script>
  import HealthStatus from './HealthStatus.svelte'

  /** @type {{ health: object | null, onretry: () => void, onlogout: () => void, checking?: boolean }} */
  let { health, onretry, onlogout, checking = false } = $props()
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" onclick={(e) => e.stopPropagation()}>
  <div class="panel">
    <div class="icon">⚠️</div>
    <h2>Services indisponibles</h2>
    <p class="sub">
      L'agent hôte ou l'engine de partages est hors ligne. Le panel est bloqué jusqu'au rétablissement des services.
    </p>

    <HealthStatus {health} compact />

    <p class="hint">RAID et SMART sur l'hôte · Partages réseau dans le conteneur engine.</p>

    <div class="actions">
      <button type="button" onclick={onretry} disabled={checking}>
        {checking ? 'Vérification…' : 'Réessayer'}
      </button>
      <button type="button" class="ghost" onclick={onlogout}>Déconnexion</button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 10000;
    display: grid;
    place-items: center;
    padding: 24px;
    background: rgba(6, 16, 24, 0.88);
    backdrop-filter: blur(6px);
  }
  .panel {
    width: min(520px, 100%);
    background: var(--bb-panel);
    border: 1px solid var(--bb-border);
    border-radius: 14px;
    padding: 28px;
    box-shadow: var(--bb-shadow);
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .icon { font-size: 36px; text-align: center; }
  h2 { font-size: 20px; text-align: center; }
  .sub { color: var(--bb-muted); font-size: 13px; text-align: center; line-height: 1.45; }
  .hint { color: var(--bb-muted); font-size: 12px; text-align: center; }
  .actions { display: flex; gap: 10px; justify-content: center; flex-wrap: wrap; }
  .actions button { min-width: 120px; }
</style>
