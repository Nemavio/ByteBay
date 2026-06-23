<script>
  import { api } from '../../lib/api.js'
  import { useDesktop } from '../../lib/desktop.js'
  import { useWindowHost } from '../../lib/windowHost.js'

  let { onSuccess = () => {} } = $props()

  const desktop = useDesktop()
  const host = useWindowHost()

  let username = $state('')
  let password = $state('')
  let web_role = $state('viewer')
  let samba_enabled = $state(false)
  let ftp_enabled = $state(false)
  let err = $state('')
  let busy = $state(false)

  function closeSelf() {
    if (host?.key) desktop?.closeWindow?.(host.key)
  }

  async function submit(e) {
    e.preventDefault()
    if (!username || !password) return
    busy = true
    err = ''
    try {
      await api.userCreate({ username, password, web_role, samba_enabled, ftp_enabled })
      onSuccess()
      closeSelf()
    } catch (e) {
      err = e.message
    } finally {
      busy = false
    }
  }
</script>

<form class="form" onsubmit={submit}>
  <div class="grid2">
    <input placeholder="Nom d'utilisateur" bind:value={username} disabled={busy} />
    <input type="password" placeholder="Mot de passe" bind:value={password} disabled={busy} />
    <select bind:value={web_role} disabled={busy}>
      <option value="admin">Web — Admin</option>
      <option value="viewer">Web — Lecture</option>
      <option value="none">Web — Aucun</option>
    </select>
    <div class="checks">
      <label class="chk"><input type="checkbox" bind:checked={samba_enabled} disabled={busy} /> Samba</label>
      <label class="chk"><input type="checkbox" bind:checked={ftp_enabled} disabled={busy} /> FTP</label>
    </div>
  </div>
  {#if err}<p class="err">{err}</p>{/if}
  <div class="actions">
    <button type="button" class="ghost" onclick={closeSelf} disabled={busy}>Annuler</button>
    <button type="submit" disabled={busy || !username || !password}>{busy ? 'Création…' : 'Créer'}</button>
  </div>
</form>

<style>
  .form { display: flex; flex-direction: column; gap: 10px; }
  .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  .checks { display: flex; gap: 12px; align-items: center; }
  .chk { flex-direction: row; font-size: 12px; color: var(--bb-muted); gap: 6px; display: flex; align-items: center; }
  .chk input { width: auto; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; }
  .err { color: var(--bb-danger); font-size: 12px; margin: 0; }
</style>
