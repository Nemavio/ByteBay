<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let users = $state([])
  let loading = $state(true)
  let msg = $state('')
  let err = $state('')

  let username = $state('')
  let password = $state('')
  let web_role = $state('viewer')
  let samba_enabled = $state(false)
  let ftp_enabled = $state(false)

  onMount(() => { load() })

  async function load() {
    loading = true
    err = ''
    try {
      users = await api.users()
    } catch (e) {
      err = e.message
    } finally {
      loading = false
    }
  }

  async function addUser() {
    msg = ''
    err = ''
    try {
      await api.userCreate({ username, password, web_role, samba_enabled, ftp_enabled })
      username = ''
      password = ''
      msg = 'Utilisateur créé et synchronisé'
      await load()
    } catch (e) {
      err = e.message
    }
  }

  async function removeUser(id) {
    if (!confirm('Supprimer ?')) return
    await api.userDelete(id)
    await load()
  }

  const roleLabel = (r) => ({ admin: 'Admin web', viewer: 'Lecture web', none: 'Pas de web' }[r] || r)
</script>

<h3>Utilisateurs unifiés</h3>
<p class="hint">
  Web (admin/viewer), Samba et FTP partagent le même compte.
  Les droits sur les dossiers se configurent dans <strong>Droits d'accès</strong>.
</p>

{#if loading}
  <p>Chargement…</p>
{:else}
  <table>
    <thead>
      <tr><th>User</th><th>Web</th><th>Samba</th><th>FTP</th><th></th></tr>
    </thead>
    <tbody>
      {#each users as u}
        <tr>
          <td>{u.username}</td>
          <td>{roleLabel(u.web_role)}</td>
          <td>{u.samba_enabled ? '✓' : '—'}</td>
          <td>{u.ftp_enabled ? '✓' : '—'}</td>
          <td><button class="danger" onclick={() => removeUser(u.id)}>×</button></td>
        </tr>
      {/each}
    </tbody>
  </table>

  <h3>Nouvel utilisateur</h3>
  <div class="form grid2">
    <input placeholder="Nom d'utilisateur" bind:value={username} />
    <input type="password" placeholder="Mot de passe" bind:value={password} />
    <select bind:value={web_role}>
      <option value="admin">Web — Admin</option>
      <option value="viewer">Web — Lecture</option>
      <option value="none">Web — Aucun</option>
    </select>
    <div class="checks">
      <label class="chk"><input type="checkbox" bind:checked={samba_enabled} /> Samba</label>
      <label class="chk"><input type="checkbox" bind:checked={ftp_enabled} /> FTP</label>
    </div>
    <button onclick={addUser} disabled={!username || !password}>Ajouter</button>
  </div>
{/if}

{#if msg}<p class="ok">{msg}</p>{/if}
{#if err}<p class="err">{err}</p>{/if}

<style>
  h3 { font-size: 13px; margin: 16px 0 8px; color: var(--bb-muted); }
  .hint { font-size: 11px; color: var(--bb-muted); margin-bottom: 10px; }
  .form { margin-top: 8px; display: flex; flex-direction: column; gap: 8px; max-width: 480px; }
  .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  .checks { display: flex; gap: 12px; align-items: center; }
  .chk { flex-direction: row; font-size: 12px; color: var(--bb-muted); gap: 6px; }
  .chk input { width: auto; }
  .ok { color: var(--bb-ok); margin-top: 8px; }
  .err { color: var(--bb-danger); margin-top: 8px; }
</style>
