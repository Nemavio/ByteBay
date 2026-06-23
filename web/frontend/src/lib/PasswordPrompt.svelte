<script>
  /** @type {{ open: boolean, title?: string, message?: string, confirmLabel?: string, onconfirm: (password: string) => Promise<void>, oncancel: () => void }} */
  let {
    open,
    title = 'Confirmer',
    message = 'Saisissez votre mot de passe pour continuer.',
    confirmLabel = 'Confirmer',
    onconfirm,
    oncancel,
  } = $props()

  let password = $state('')
  let error = $state('')
  let busy = $state(false)

  $effect(() => {
    if (open) {
      password = ''
      error = ''
      busy = false
    }
  })

  async function submit(e) {
    e.preventDefault()
    if (!password) {
      error = 'Mot de passe requis'
      return
    }
    busy = true
    error = ''
    try {
      await onconfirm(password)
      password = ''
    } catch (err) {
      error = err?.message || 'Échec de la confirmation'
    } finally {
      busy = false
    }
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
      <p class="msg">{message}</p>
      <label>
        Mot de passe
        <input type="password" bind:value={password} autocomplete="current-password" required disabled={busy} />
      </label>
      {#if error}<p class="err">{error}</p>{/if}
      <div class="actions">
        <button type="button" class="ghost" onclick={oncancel} disabled={busy}>Annuler</button>
        <button type="submit" class="danger" disabled={busy}>{busy ? 'Vérification…' : confirmLabel}</button>
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
    width: min(360px, 100%);
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
  .err { color: var(--bb-danger); font-size: 12px; }
</style>
