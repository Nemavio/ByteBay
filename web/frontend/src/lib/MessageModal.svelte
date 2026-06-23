<script>
  /** @type {{ open: boolean, title?: string, message?: string, variant?: 'info' | 'error' | 'success', onclose: () => void }} */
  let {
    open,
    title = 'Information',
    message = '',
    variant = 'info',
    onclose,
  } = $props()
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="overlay" onclick={onclose}>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="panel" class:error={variant === 'error'} class:success={variant === 'success'} onclick={(e) => e.stopPropagation()}>
      <div class="icon" aria-hidden="true">
        {#if variant === 'error'}⚠️{:else if variant === 'success'}✓{:else}ℹ️{/if}
      </div>
      <h3>{title}</h3>
      <p class="msg">{message}</p>
      <div class="actions">
        <button type="button" onclick={onclose}>Fermer</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 20000;
    display: grid;
    place-items: center;
    padding: 24px;
    background: rgba(6, 16, 24, 0.75);
    backdrop-filter: blur(4px);
  }
  .panel {
    width: min(440px, 100%);
    background: var(--bb-panel);
    border: 1px solid var(--bb-border);
    border-radius: 12px;
    padding: 22px;
    box-shadow: var(--bb-shadow);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .panel.error { border-left: 3px solid var(--bb-danger); }
  .panel.success { border-left: 3px solid var(--bb-ok); }
  .icon { font-size: 28px; text-align: center; }
  h3 { font-size: 16px; text-align: center; }
  .msg {
    color: var(--bb-text);
    font-size: 13px;
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .actions { display: flex; justify-content: center; margin-top: 4px; }
</style>
