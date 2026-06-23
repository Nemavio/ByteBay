<script>
  import { api, getToken, setToken } from './lib/api.js'

  let { onLogin } = $props()
  let username = $state('admin')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)

  async function submit(e) {
    e.preventDefault()
    loading = true
    error = ''
    try {
      const { access_token } = await api.login(username, password)
      setToken(access_token)
      onLogin()
    } catch {
      error = 'Identifiants invalides'
    } finally {
      loading = false
    }
  }
</script>

<div class="login-screen">
  <form class="card" onsubmit={submit}>
    <div class="logo">📦</div>
    <h1>ByteBay</h1>
    <p class="sub">Connexion au panel d'administration</p>
    <label>
      Utilisateur
      <input name="username" bind:value={username} autocomplete="username" required />
    </label>
    <label>
      Mot de passe
      <input name="password" type="password" bind:value={password} autocomplete="current-password" required />
    </label>
    {#if error}<p class="err">{error}</p>{/if}
    <button type="submit" disabled={loading}>{loading ? 'Connexion…' : 'Se connecter'}</button>
  </form>
</div>

<style>
  .login-screen {
    height: 100%;
    display: grid;
    place-items: center;
    overflow: visible;
    position: relative;
    z-index: 0;
    background:
      radial-gradient(ellipse at 30% 20%, #1a3a5c 0%, transparent 50%),
      radial-gradient(ellipse at 70% 80%, #0d2840 0%, transparent 50%),
      var(--bb-bg);
  }
  .card {
    width: min(360px, 92vw);
    background: var(--bb-panel);
    border: 1px solid var(--bb-border);
    border-radius: 14px;
    padding: 28px;
    box-shadow: var(--bb-shadow);
    display: flex;
    flex-direction: column;
    gap: 12px;
    position: relative;
    z-index: 1;
    overflow: visible;
  }
  .logo { font-size: 40px; text-align: center; }
  h1 { text-align: center; font-size: 22px; }
  .sub { text-align: center; color: var(--bb-muted); font-size: 13px; margin-bottom: 8px; }
  label { display: flex; flex-direction: column; gap: 6px; font-size: 12px; color: var(--bb-muted); }
  .err { color: var(--bb-danger); font-size: 13px; text-align: center; }
  button { margin-top: 4px; width: 100%; padding: 10px; }
</style>
