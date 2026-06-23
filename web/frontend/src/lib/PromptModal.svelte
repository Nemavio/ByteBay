<script>
  /** @type {{ open: boolean, title?: string, message?: string, label?: string, placeholder?: string, confirmLabel?: string, initialValue?: string, onconfirm: (value: string) => void, oncancel: () => void }} */
  let {
    open,
    title = 'Saisie',
    message = '',
    label = 'Valeur',
    placeholder = '',
    confirmLabel = 'OK',
    initialValue = '',
    onconfirm,
    oncancel,
  } = $props()

  let value = $state('')
  let error = $state('')

  $effect(() => {
    if (open) {
      value = initialValue
      error = ''
    }
  })

  function submit(e) {
    e.preventDefault()
    const v = value.trim()
    if (!v) {
      error = 'Champ requis'
      return
    }
    onconfirm(v)
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="overlay" onclick={oncancel}>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <form class="panel" onclick={(e) => e.stopPropagation()} onsubmit={submit}>
      <h3>{title}</h3>
      {#if message}<p class="msg">{message}</p>{/if}
      <label>
        {label}
        <input type="text" bind:value={value} {placeholder} autocomplete="off" />
      </label>
      {#if error}<p class="err">{error}</p>{/if}
      <div class="actions">
        <button type="button" class="ghost" onclick={oncancel}>Annuler</button>
        <button type="submit">{confirmLabel}</button>
      </div>
    </form>
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
    width: min(400px, 100%);
    background: var(--bb-panel);
    border: 1px solid var(--bb-border);
    border-radius: 12px;
    padding: 22px;
    box-shadow: var(--bb-shadow);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  h3 { font-size: 16px; }
  .msg { color: var(--bb-muted); font-size: 13px; line-height: 1.45; }
  label { display: flex; flex-direction: column; gap: 6px; font-size: 12px; color: var(--bb-muted); }
  .actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 4px; }
  .ghost {
    background: transparent;
    border: 1px solid var(--bb-border);
  }
  .err { color: var(--bb-danger); font-size: 12px; }
</style>
