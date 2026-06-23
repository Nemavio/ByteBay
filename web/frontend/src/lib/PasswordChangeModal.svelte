<script>
  /** @type {{ open: boolean, title?: string, message?: string, username?: string, confirmLabel?: string, onconfirm: (password: string) => Promise<void>, oncancel: () => void }} */
  let {
    open,
    title = 'Changer le mot de passe',
    message = '',
    username = '',
    confirmLabel = 'Enregistrer',
    onconfirm,
    oncancel,
  } = $props()

  let password = $state('')
  let confirmPwd = $state('')
  let error = $state('')
  let busy = $state(false)

  $effect(() => {
    if (open) {
      password = ''
      confirmPwd = ''
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
    if (password.length < 4) {
      error = 'Au moins 4 caractères'
      return
    }
    if (password !== confirmPwd) {
      error = 'Les mots de passe ne correspondent pas'
      return
    }
    busy = true
    error = ''
    try {
      await onconfirm(password)
    } catch (e) {
      error = e.message || 'Erreur'
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
      {#if message}<p class="msg">{message}</p>{/if}
      {#if username}<p class="user">{username}</p>{/if}
      <label>
        Nouveau mot de passe
        <input type="password" bind:value={password} autocomplete="new-password" required disabled={busy} />
      </label>
      <label>
        Confirmer
        <input type="password" bind:value={confirmPwd} autocomplete="new-password" required disabled={busy} />
      </label>
      {#if error}<p class="err">{error}</p>{/if}
      <div class="actions">
        <button type="button" class="ghost" onclick={oncancel} disabled={busy}>Annuler</button>
        <button type="submit" disabled={busy}>{confirmLabel}</button>
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
  .user { font-size: 13px; font-weight: 600; }
  label { display: flex; flex-direction: column; gap: 6px; font-size: 12px; color: var(--bb-muted); }
  .actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 4px; }
  .ghost {
    background: transparent;
    border: 1px solid var(--bb-border);
  }
  .err { color: var(--bb-danger); font-size: 12px; }
</style>
