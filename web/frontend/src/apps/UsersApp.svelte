<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { useDesktop } from '../lib/desktop.js'
  import { useWindowHost } from '../lib/windowHost.js'
  import ConfirmModal from '../lib/ConfirmModal.svelte'
  import PasswordChangeModal from '../lib/PasswordChangeModal.svelte'
  import UserCreateForm from './forms/UserCreateForm.svelte'

  const desktop = useDesktop()
  const host = useWindowHost()

  let users = $state([])
  let loading = $state(true)
  let msg = $state('')
  let err = $state('')

  let confirm = $state({ open: false, title: '', message: '', variant: 'danger', confirmLabel: 'Supprimer', onOk: null })
  let pwdModal = $state({ open: false, user: null })

  function openConfirm({ title, message, variant = 'danger', confirmLabel = 'Supprimer', onOk }) {
    confirm = { open: true, title, message, variant, confirmLabel, onOk }
  }

  function closeConfirm() {
    confirm = { ...confirm, open: false, onOk: null }
  }

  function handleConfirm() {
    const fn = confirm.onOk
    closeConfirm()
    fn?.()
  }

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

  function openAddForm() {
    msg = ''
    err = ''
    desktop?.openCustomWindow({
      title: 'Nouvel utilisateur',
      component: UserCreateForm,
      props: { onSuccess: () => { msg = 'Utilisateur créé et synchronisé'; load() } },
      w: 500,
      h: 300,
      from: host,
    })
  }

  function removeUser(id) {
    openConfirm({
      title: 'Supprimer l\'utilisateur',
      message: 'Supprimer cet utilisateur ? Cette action est irréversible.',
      onOk: async () => {
        try {
          await api.userDelete(id)
          await load()
        } catch (e) {
          err = e.message
        }
      },
    })
  }

  function openPasswordModal(u) {
    msg = ''
    err = ''
    pwdModal = { open: true, user: u }
  }

  function closePasswordModal() {
    pwdModal = { open: false, user: null }
  }

  async function changePassword(password) {
    const u = pwdModal.user
    if (!u) return
    await api.userUpdate(u.id, { password })
    closePasswordModal()
    msg = `Mot de passe de « ${u.username} » mis à jour (web, Samba, FTP)`
    await load()
  }

  const roleLabel = (r) => ({ admin: 'Admin web', viewer: 'Lecture web', none: 'Pas de web' }[r] || r)
</script>

<h3>Utilisateurs unifiés</h3>
<p class="hint">
  Web (admin/viewer), Samba et FTP partagent le même compte.
  Pour Samba ou FTP depuis Windows / FileZilla : cochez <strong>Samba</strong> ou <strong>FTP</strong> et définissez un mot de passe (obligatoire à l'activation).
  Dans <strong>Partages → FTP</strong>, le champ « Utilisateur FTP » doit être <strong>identique</strong> au nom du compte.
  Les droits sur les dossiers se configurent dans la fenêtre
  <button type="button" class="link" onclick={() => desktop?.openAppById('access')}>Droits d'accès</button>.
</p>

{#if loading}
  <p>Chargement…</p>
{:else}
  <div class="section-head">
    <span></span>
    <button type="button" onclick={openAddForm}>+ Nouvel utilisateur</button>
  </div>
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
          <td class="row-actions">
            <button type="button" class="ghost tiny" title="Changer le mot de passe" onclick={() => openPasswordModal(u)}>🔑</button>
            <button type="button" class="danger" title="Supprimer" onclick={() => removeUser(u.id)}>×</button>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

{#if msg}<p class="ok">{msg}</p>{/if}
{#if err}<p class="err">{err}</p>{/if}

<ConfirmModal
  open={confirm.open}
  title={confirm.title}
  message={confirm.message}
  variant={confirm.variant}
  confirmLabel={confirm.confirmLabel}
  onconfirm={handleConfirm}
  oncancel={closeConfirm}
/>

<PasswordChangeModal
  open={pwdModal.open}
  username={pwdModal.user?.username ?? ''}
  message="Le mot de passe s'applique au panneau web, Samba et FTP pour ce compte."
  onconfirm={changePassword}
  oncancel={closePasswordModal}
/>

<style>
  h3 { font-size: 13px; margin: 16px 0 8px; color: var(--bb-muted); }
  .hint { font-size: 11px; color: var(--bb-muted); margin-bottom: 10px; }
  .link {
    background: none;
    border: none;
    padding: 0;
    color: var(--bb-accent);
    font-size: inherit;
    text-decoration: underline;
  }
  .link:hover { color: var(--bb-accent-dim); }
  .section-head { display: flex; justify-content: flex-end; margin-bottom: 8px; }
  .ok { color: var(--bb-ok); margin-top: 8px; }
  .err { color: var(--bb-danger); margin-top: 8px; }
  .row-actions { display: flex; gap: 4px; justify-content: flex-end; white-space: nowrap; }
  .tiny { padding: 2px 8px; font-size: 12px; }
</style>
