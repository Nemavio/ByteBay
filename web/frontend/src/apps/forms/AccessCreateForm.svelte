<script>
  import { onMount } from 'svelte'
  import { api } from '../../lib/api.js'
  import { useDesktop } from '../../lib/desktop.js'
  import { useWindowHost } from '../../lib/windowHost.js'
  import PathBrowser from '../../lib/PathBrowser.svelte'

  let { onSuccess = () => {} } = $props()

  const desktop = useDesktop()
  const host = useWindowHost()

  let users = $state([])
  let aclPath = $state('/volumes')
  let aclUser = $state('')
  let aclRead = $state(true)
  let aclWrite = $state(false)
  let err = $state('')
  let busy = $state(false)

  onMount(async () => {
    try {
      users = await api.users()
      if (users.length) aclUser = users[0].username
    } catch (e) {
      err = e.message
    }
  })

  function closeSelf() {
    if (host?.key) desktop?.closeWindow?.(host.key)
  }

  async function submit(e) {
    e.preventDefault()
    const p = aclPath.trim()
    if (!p.startsWith('/volumes') && !p.startsWith('/data')) {
      err = 'Le chemin doit commencer par /volumes ou /data'
      return
    }
    busy = true
    err = ''
    try {
      await api.aclCreate({
        path: p,
        username: aclUser,
        can_read: aclRead,
        can_write: aclWrite,
      })
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
  <PathBrowser bind:path={aclPath} height="220px" />
  <div class="grid2">
    <label>Utilisateur
      <select bind:value={aclUser} disabled={busy}>
        {#each users as u}
          <option value={u.username}>{u.username}</option>
        {/each}
      </select>
    </label>
    <div class="perms">
      <label class="chk"><input type="checkbox" bind:checked={aclRead} disabled={busy} /> Lecture</label>
      <label class="chk"><input type="checkbox" bind:checked={aclWrite} disabled={busy} /> Écriture</label>
    </div>
  </div>
  {#if err}<p class="err">{err}</p>{/if}
  <div class="actions">
    <button type="button" class="ghost" onclick={closeSelf} disabled={busy}>Annuler</button>
    <button type="submit" disabled={busy || !aclPath.trim() || !aclUser}>{busy ? 'Ajout…' : 'Ajouter'}</button>
  </div>
</form>

<style>
  .form { display: flex; flex-direction: column; gap: 10px; }
  .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; align-items: end; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--bb-muted); }
  .perms { display: flex; flex-direction: column; gap: 6px; }
  .chk { flex-direction: row; align-items: center; gap: 6px; color: var(--bb-text); display: flex; }
  .chk input { width: auto; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; }
  .err { color: var(--bb-danger); font-size: 12px; margin: 0; }
</style>
