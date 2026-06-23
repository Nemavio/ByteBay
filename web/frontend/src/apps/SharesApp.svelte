<script>
  import { onMount, tick } from 'svelte'
  import { api } from '../lib/api.js'
  import PathBrowser from '../lib/PathBrowser.svelte'

  let tab = $state('nfs')
  let cfg = $state({ nfs: [], samba: [], ftp: [] })
  let msg = $state('')
  let err = $state('')
  let applying = $state(false)
  /** @type {number | null} */
  let browseRow = $state(null)
  /** @type {HTMLElement | null} */
  let listEnd = $state(null)

  const empty = {
    nfs: { export: '', path: '/volumes', clients: '192.168.0.0/16', options: 'rw,async,no_subtree_check,no_root_squash', enabled: true },
    samba: { name: 'public', path: '/volumes', browseable: true, read_only: false, guest_ok: false, enabled: true },
    ftp: { name: 'uploads', path: '/volumes', enabled: true },
  }

  onMount(() => {
    load()
  })

  $effect(() => {
    tab
    browseRow = null
  })

  function normalizeCfg(data) {
    /** @type {typeof cfg} */
    const next = { nfs: [], samba: [], ftp: [] }
    for (const k of ['nfs', 'samba', 'ftp']) {
      next[k] = data[k]?.length ? [...data[k]] : []
    }
    return next
  }

  function newRow(kind) {
    const row = { ...empty[kind] }
    const n = (cfg[kind]?.length ?? 0) + 1
    if (kind === 'samba') row.name = `partage${n}`
    if (kind === 'ftp') row.name = `user${n}`
    if (kind === 'nfs') row.export = `export${n}`
    return row
  }

  /** Chemin NFS visible par les clients (serveur:chemin). */
  function nfsMountPath(row) {
    const exp = (row.export || '').trim()
    if (!exp) return row.path || '—'
    if (exp.startsWith('/')) return exp
    return `/${exp}`
  }

  const nfsExportNameRe = /^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/

  function validateNFS(rows) {
    const pathErr = validatePaths(rows)
    if (pathErr) return pathErr
    const seen = new Set()
    for (const row of rows) {
      const exp = (row.export || '').trim()
      if (!exp) continue
      if (exp.includes('/')) {
        const clean = exp.replace(/\/+$/, '') || '/'
        if (!/^\/[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(clean)) {
          return 'Point de montage invalide : nom court (ex. backup) ou /backup'
        }
        if (seen.has(clean)) return `Point de montage NFS en double : ${clean}`
        seen.add(clean)
        continue
      }
      if (!nfsExportNameRe.test(exp)) {
        return `Nom NFS invalide « ${exp} » : lettres, chiffres, - ou _`
      }
      const mount = `/${exp}`
      if (seen.has(mount)) return `Point de montage NFS en double : ${mount}`
      seen.add(mount)
    }
    return ''
  }

  async function load() {
    try {
      cfg = normalizeCfg(await api.shares())
    } catch (e) {
      err = e.message
    }
  }

  async function addRow() {
    const rows = [...(cfg[tab] || []), newRow(tab)]
    cfg = { ...cfg, [tab]: rows }
    browseRow = rows.length - 1
    await tick()
    listEnd?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }

  function removeRow(i) {
    const rows = (cfg[tab] || []).filter((_, j) => j !== i)
    cfg = { ...cfg, [tab]: rows }
    browseRow = null
  }

  function toggleBrowse(i) {
    browseRow = browseRow === i ? null : i
  }

  function validatePaths(rows) {
    if (!rows.length) return ''
    for (const row of rows) {
      const p = (row.path || '').trim()
      if (!p.startsWith('/volumes') && !p.startsWith('/data')) {
        return 'Chaque chemin doit commencer par /volumes ou /data'
      }
    }
    return ''
  }

  async function save() {
    msg = ''
    err = ''
    const v = tab === 'nfs' ? validateNFS(cfg[tab]) : validatePaths(cfg[tab])
    if (v) {
      err = v
      return
    }
    try {
      cfg = normalizeCfg(await api.sharesPut(tab, cfg[tab]))
      msg = 'Partages enregistrés et services rechargés'
      browseRow = null
    } catch (e) {
      err = e.message
    }
  }

  async function reapply() {
    applying = true
    msg = ''
    err = ''
    try {
      const res = await api.sharesApply()
      msg = `NFS: ${res.nfs} · Samba: ${res.samba} · FTP: ${res.ftp}`
    } catch (e) {
      err = e.message
    } finally {
      applying = false
    }
  }
</script>

<p class="hint">
  Chemins sous <code>/volumes/…</code> (montés depuis l'hôte via <strong>Montages</strong>).
  Utilisez <strong>Parcourir</strong> pour choisir le dossier réel. NFS (Ganesha) : le <strong>point de montage</strong> (ex. <code>backup</code> → <code>serveur:/backup</code>) est le chemin client v3/v4.
  ACL par IP (champ Clients). <code>no_root_squash</code> pour rsync root (réseau de confiance).
  Samba : laissez <strong>Invité</strong> décoché pour une connexion avec identifiants (Utilisateurs → Samba).
  FTP : le champ <strong>Utilisateur FTP</strong> doit correspondre au login défini dans <strong>Utilisateurs</strong> (FTP coché).
</p>

<div class="tabs">
  {#each ['nfs', 'samba', 'ftp'] as t}
    <button type="button" class:active={tab === t} onclick={() => (tab = t)}>{t.toUpperCase()}</button>
  {/each}
  <button type="button" class="ghost" onclick={reapply} disabled={applying}>Recharger services</button>
</div>

{#if tab === 'nfs'}
  {#if !cfg.nfs.length}
    <p class="empty">Aucun export NFS. Cliquez sur <strong>+ Partage</strong> pour en ajouter un, ou <strong>Enregistrer</strong> pour désactiver NFS.</p>
  {/if}
  {#each cfg.nfs as row, i}
    <article class="share-block">
      <div class="share-head">
        <span class="share-title">Export NFS {i + 1}</span>
        <button type="button" class="ghost tiny" onclick={() => removeRow(i)} title="Supprimer">×</button>
      </div>
      <div class="path-line">
        <span class="path-label">Dossier</span>
        <code class="path-val">{row.path || '—'}</code>
        <button type="button" class="ghost tiny browse-btn" class:active={browseRow === i} onclick={() => toggleBrowse(i)}>
          {browseRow === i ? 'Masquer' : 'Parcourir…'}
        </button>
      </div>
      {#if browseRow === i}
        <PathBrowser bind:path={row.path} height="200px" />
      {/if}
      <div class="fields grid-nfs">
        <label>Point de montage NFS
          <input placeholder="backup" bind:value={row.export} />
          <span class="field-hint">Clients : <code>{nfsMountPath(row)}</code></span>
        </label>
        <label>Clients (réseau)
          <input placeholder="192.168.0.0/16" bind:value={row.clients} />
        </label>
        <label>Options
          <input placeholder="rw,async,no_subtree_check" bind:value={row.options} />
        </label>
        <label class="chk"><input type="checkbox" bind:checked={row.enabled} /> Actif</label>
      </div>
    </article>
  {/each}
{:else if tab === 'samba'}
  {#if !cfg.samba.length}
    <p class="empty">Aucun partage Samba. Cliquez sur <strong>+ Partage</strong> pour en ajouter un, ou <strong>Enregistrer</strong> pour tout désactiver.</p>
  {/if}
  {#each cfg.samba as row, i}
    <article class="share-block">
      <div class="share-head">
        <span class="share-title">Partage Samba {i + 1}</span>
        <button type="button" class="ghost tiny" onclick={() => removeRow(i)} title="Supprimer">×</button>
      </div>
      <div class="fields grid-samba">
        <label>Nom du partage
          <input placeholder="public" bind:value={row.name} />
        </label>
      </div>
      <div class="path-line">
        <code class="path-val">{row.path || '—'}</code>
        <button type="button" class="ghost tiny browse-btn" class:active={browseRow === i} onclick={() => toggleBrowse(i)}>
          {browseRow === i ? 'Masquer' : 'Parcourir…'}
        </button>
      </div>
      {#if browseRow === i}
        <PathBrowser bind:path={row.path} height="200px" />
      {/if}
      <div class="checks">
        <label class="chk"><input type="checkbox" bind:checked={row.browseable} /> Navigable</label>
        <label class="chk"><input type="checkbox" bind:checked={row.read_only} /> Lecture seule</label>
        <label class="chk"><input type="checkbox" bind:checked={row.guest_ok} /> Invité</label>
        <label class="chk"><input type="checkbox" bind:checked={row.enabled} /> Actif</label>
      </div>
    </article>
  {/each}
{:else}
  {#if !cfg.ftp.length}
    <p class="empty">Aucun compte FTP. Cliquez sur <strong>+ Partage</strong> pour en ajouter un, ou <strong>Enregistrer</strong> pour tout désactiver.</p>
  {/if}
  {#each cfg.ftp as row, i}
    <article class="share-block">
      <div class="share-head">
        <span class="share-title">Compte FTP {i + 1}</span>
        <button type="button" class="ghost tiny" onclick={() => removeRow(i)} title="Supprimer">×</button>
      </div>
      <div class="fields grid-ftp">
        <label>Utilisateur FTP
          <input placeholder="uploads" bind:value={row.name} />
        </label>
      </div>
      <div class="path-line">
        <code class="path-val">{row.path || '—'}</code>
        <button type="button" class="ghost tiny browse-btn" class:active={browseRow === i} onclick={() => toggleBrowse(i)}>
          {browseRow === i ? 'Masquer' : 'Parcourir…'}
        </button>
      </div>
      {#if browseRow === i}
        <PathBrowser bind:path={row.path} height="200px" />
      {/if}
      <label class="chk"><input type="checkbox" bind:checked={row.enabled} /> Actif</label>
    </article>
  {/each}
{/if}

<div bind:this={listEnd} class="list-end" aria-hidden="true"></div>

<div class="actions">
  <button type="button" class="ghost" onclick={addRow}>+ Partage</button>
  <button type="button" onclick={save}>Enregistrer</button>
</div>
{#if msg}<p class="ok">{msg}</p>{/if}
{#if err}<p class="err">{err}</p>{/if}

<style>
  .hint { font-size: 11px; color: var(--bb-muted); margin-bottom: 10px; line-height: 1.45; }
  .tabs { display: flex; gap: 6px; margin-bottom: 12px; flex-wrap: wrap; }
  .tabs button { background: var(--bb-panel); font-size: 12px; padding: 6px 10px; }
  .tabs button.active { background: var(--bb-accent); }
  .share-block {
    border: 1px solid var(--bb-border);
    border-radius: 8px;
    padding: 10px 12px;
    margin-bottom: 10px;
    background: rgba(0, 0, 0, 0.12);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .share-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .share-title { font-size: 12px; font-weight: 600; color: var(--bb-muted); }
  .path-line {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .path-val {
    flex: 1;
    min-width: 120px;
    font-size: 11px;
    padding: 5px 8px;
    background: rgba(0, 0, 0, 0.2);
    border-radius: 4px;
    border: 1px solid var(--bb-border);
    word-break: break-all;
  }
  .browse-btn.active { background: var(--bb-accent); }
  .fields { display: grid; gap: 8px; }
  .grid-nfs { grid-template-columns: 1fr 1fr 1fr auto; align-items: end; }
  .path-label { font-size: 0.75rem; color: var(--muted); margin-right: 0.35rem; }
  .field-hint { display: block; font-size: 0.72rem; color: var(--muted); margin-top: 0.2rem; }
  .field-hint code { font-size: 0.72rem; }
  .grid-samba, .grid-ftp { grid-template-columns: 1fr; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 11px; color: var(--bb-muted); }
  .checks { display: flex; flex-wrap: wrap; gap: 10px 14px; }
  .chk { flex-direction: row; align-items: center; gap: 6px; color: var(--bb-text); white-space: nowrap; }
  .chk input { width: auto; }
  .tiny { padding: 2px 8px; font-size: 11px; }
  .actions { display: flex; gap: 8px; margin-top: 12px; }
  .list-end { height: 0; }
  .empty {
    font-size: 12px;
    color: var(--bb-muted);
    margin: 0 0 12px;
    padding: 12px;
    border: 1px dashed var(--bb-border);
    border-radius: 8px;
    line-height: 1.45;
  }
  .ok { color: var(--bb-ok); margin-top: 8px; font-size: 12px; }
  .err { color: var(--bb-danger); margin-top: 8px; font-size: 12px; }
</style>
