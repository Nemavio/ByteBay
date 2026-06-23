<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { useDesktop } from '../lib/desktop.js'
  import { useWindowHost } from '../lib/windowHost.js'
  import ConfirmModal from '../lib/ConfirmModal.svelte'
  import AccessCreateForm from './forms/AccessCreateForm.svelte'

  const desktop = useDesktop()
  const host = useWindowHost()

  let acl = $state([])
  let loading = $state(true)
  let msg = $state('')
  let err = $state('')

  let confirm = $state({ open: false, title: '', message: '', variant: 'danger', confirmLabel: 'Retirer', onOk: null })

  function openConfirm({ title, message, onOk }) {
    confirm = { open: true, title, message, variant: 'danger', confirmLabel: 'Retirer', onOk }
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
      acl = await api.acl()
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
      title: 'Ajouter un droit',
      component: AccessCreateForm,
      props: { onSuccess: () => { msg = 'Droit ajouté'; load() } },
      w: 560,
      h: 480,
      from: host,
    })
  }

  function removeAcl(id) {
    openConfirm({
      title: 'Retirer le droit',
      message: 'Retirer ce droit d\'accès ?',
      onOk: async () => {
        try {
          await api.aclDelete(id)
          await load()
        } catch (e) {
          err = e.message
        }
      },
    })
  }
</script>

<h3>Droits d'accès</h3>
<p class="hint">
  Contrôle d'accès aux dossiers pour l'explorateur web, Samba et FTP.
  Les admins web ont accès à tout. NFS : ACL par IP dans Partages.
</p>

{#if loading}
  <p>Chargement…</p>
{:else}
  <div class="section-head">
    <span></span>
    <button type="button" onclick={openAddForm}>+ Ajouter un droit</button>
  </div>
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

<style>
  h3 { font-size: 13px; margin: 16px 0 8px; color: var(--bb-muted); font-weight: 600; }
  h3:first-of-type { margin-top: 0; }
  .hint { font-size: 11px; color: var(--bb-muted); margin-bottom: 10px; }
  .muted { color: var(--bb-muted); font-size: 12px; }
  .section-head { display: flex; justify-content: flex-end; margin-bottom: 8px; }
  code { font-size: 11px; }
  .ok { color: var(--bb-ok); margin-top: 8px; }
  .err { color: var(--bb-danger); margin-top: 8px; }
</style>
