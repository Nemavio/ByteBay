<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let acl = $state([])
  let users = $state([])
  let loading = $state(true)
  let msg = $state('')
  let err = $state('')

  let aclPath = $state('/volumes/raid1/public')
  let aclUser = $state('')
  let aclRead = $state(true)
  let aclWrite = $state(false)

  onMount(() => { load() })

  async function load() {
    loading = true
    err = ''
    try {
      const [a, u] = await Promise.all([api.acl(), api.users()])
      acl = a
      users = u
      if (!aclUser && u.length) aclUser = u[0].username
    } catch (e) {
      err = e.message
    } finally {
      loading = false
    }
  }

  async function addAcl() {
    msg = ''
    err = ''
    try {
      await api.aclCreate({
        path: aclPath,
        username: aclUser,
        can_read: aclRead,
        can_write: aclWrite,
      })
      msg = 'Droit ajouté'
      await load()
    } catch (e) {
      err = e.message
    }
  }

  async function removeAcl(id) {
    if (!confirm('Retirer ce droit ?')) return
    await api.aclDelete(id)
    await load()
  }
</script>

<p class="hint">
  Contrôle d'accès aux dossiers pour l'explorateur web, Samba et FTP.
  Les admins web ont accès à tout. NFS : ACL par IP dans Partages.
</p>

{#if loading}
  <p>Chargement…</p>
{:else}
  <table>
    <thead>
      <tr><th>Chemin</th><th>Utilisateur</th><th>Lecture</th><th>Écriture</th><th></th></tr>
    </thead>
    <tbody>
      {#if acl.length === 0}
        <tr><td colspan="5" class="muted">Aucun droit configuré.</td></tr>
      {:else}
        {#each acl as row}
          <tr>
            <td><code>{row.path}</code></td>
            <td>{row.username}</td>
            <td>{row.can_read ? '✓' : '—'}</td>
            <td>{row.can_write ? '✓' : '—'}</td>
            <td><button class="ghost" onclick={() => removeAcl(row.id)}>×</button></td>
          </tr>
        {/each}
      {/if}
    </tbody>
  </table>

  <h3>Ajouter un droit</h3>
  <div class="form grid2">
    <label>Chemin
      <input placeholder="/volumes/raid1/public" bind:value={aclPath} />
    </label>
    <label>Utilisateur
      <select bind:value={aclUser}>
        {#each users as u}
          <option value={u.username}>{u.username}</option>
        {/each}
      </select>
    </label>
    <label class="chk"><input type="checkbox" bind:checked={aclRead} /> Lecture</label>
    <label class="chk"><input type="checkbox" bind:checked={aclWrite} /> Écriture</label>
    <button onclick={addAcl} disabled={!aclPath || !aclUser}>Ajouter</button>
  </div>
{/if}

{#if msg}<p class="ok">{msg}</p>{/if}
{#if err}<p class="err">{err}</p>{/if}

<style>
  h3 { font-size: 13px; margin: 16px 0 8px; color: var(--bb-muted); }
  .hint { font-size: 11px; color: var(--bb-muted); margin-bottom: 10px; }
  .muted { color: var(--bb-muted); font-size: 12px; }
  .form { margin-top: 8px; display: flex; flex-direction: column; gap: 8px; max-width: 520px; }
  .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--bb-muted); }
  .chk { flex-direction: row; align-items: center; gap: 6px; color: var(--bb-text); }
  .chk input { width: auto; }
  code { font-size: 11px; }
  .ok { color: var(--bb-ok); margin-top: 8px; }
  .err { color: var(--bb-danger); margin-top: 8px; }
</style>
