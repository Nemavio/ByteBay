<script>
  /** @type {{ open: boolean, title?: string, message?: string, variant?: 'info' | 'warn' | 'danger', confirmLabel?: string, cancelLabel?: string, onconfirm: () => void, oncancel: () => void }} */
  let {
    open,
    title = 'Confirmer',
    message = '',
    variant = 'warn',
    confirmLabel = 'Confirmer',
    cancelLabel = 'Annuler',
    onconfirm,
    oncancel,
  } = $props()
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="overlay" onclick={oncancel}>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="panel"
      class:warn={variant === 'warn'}
      class:danger={variant === 'danger'}
      onclick={(e) => e.stopPropagation()}
    >
      <div class="icon" aria-hidden="true">
        {#if variant === 'danger'}⚠️{:else if variant === 'warn'}❓{:else}ℹ️{/if}
      </div>
      <h3>{title}</h3>
      <p class="msg">{message}</p>
      <div class="actions">
        <button type="button" class="ghost" onclick={oncancel}>{cancelLabel}</button>
        <button type="button" class:confirm-danger={variant === 'danger'} onclick={onconfirm}>{confirmLabel}</button>
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
  .panel.warn { border-left: 3px solid #f0c674; }
  .panel.danger { border-left: 3px solid var(--bb-danger); }
  .icon { font-size: 28px; text-align: center; }
  h3 { font-size: 16px; text-align: center; }
  .msg {
    color: var(--bb-text);
    font-size: 13px;
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-word;
    text-align: center;
  }
  .actions { display: flex; gap: 8px; justify-content: center; margin-top: 4px; }
  .ghost {
    background: transparent;
    border: 1px solid var(--bb-border);
  }
  .confirm-danger {
    background: var(--bb-danger);
    border-color: var(--bb-danger);
  }
</style>
