<script>
  import { onMount } from 'svelte'
  import { getToken } from '../lib/api.js'
  import * as webdav from '../lib/webdav.js'
  import { previewKind, blobForPreview, isHeic, heicToPreviewUrl } from '../lib/mediaPreview.js'
  import { useContextMenu, openContextMenu } from '../lib/contextMenu.js'
  import PromptModal from '../lib/PromptModal.svelte'
  import ConfirmModal from '../lib/ConfirmModal.svelte'

  const ctxMenu = useContextMenu()

  const ROOTS = [
    { path: '/volumes', label: 'Volumes' },
    { path: '/data', label: 'Système' },
  ]

  let path = $state('/volumes')
  let entries = $state([])
  let loading = $state(false)
  let error = $state('')
  let preview = $state(null)
  let previewUrl = $state('')
  let previewText = $state('')
  let previewError = $state('')
  let loadingPreview = $state(false)
  let uploading = $state(false)
  let uploadPct = $state(0)
  let uploadLabel = $state('')

  /** @type {Record<string, { name: string, path: string }[]>} */
  let treeCache = $state({})
  /** @type {Set<string>} */
  let expanded = $state(new Set(['/volumes', '/data']))

  let mkdirOpen = $state(false)
  let renameOpen = $state(false)
  /** @type {{ name: string, path: string, is_dir: boolean } | null} */
  let renameEntry = $state(null)
  let deleteOpen = $state(false)
  /** @type {{ name: string, path: string, is_dir: boolean } | null} */
  let deleteEntry = $state(null)
  /** @type {{ name: string, path: string, is_dir: boolean }[]} */
  let deleteTargets = $state([])

  /** @type {Set<string>} */
  let selectedPaths = $state(new Set())
  /** Dernier index cliqué pour la sélection par plage (Shift). */
  let selectionAnchor = $state(null)

  onMount(() => {
    ROOTS.forEach((r) => loadTreeChildren(r.path))
    browse('/volumes')
  })

  function parentPath(p) {
    if (ROOTS.some((r) => r.path === p)) return null
    const i = p.lastIndexOf('/')
    if (i <= 0) return '/'
    return p.slice(0, i) || '/'
  }

  async function browse(p, { expandTree = true } = {}) {
    loading = true
    error = ''
    closePreview()
    selectedPaths = new Set()
    selectionAnchor = null
    try {
      const list = await webdav.listDirectory(p)
      entries = list
      path = p
      if (expandTree) {
        await ensureExpandedTo(p)
      }
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  async function ensureExpandedTo(target) {
    const parts = target.split('/').filter(Boolean)
    let cur = ''
    const next = new Set(expanded)
    for (const part of parts) {
      cur += '/' + part
      next.add(cur)
      if (!treeCache[cur]) await loadTreeChildren(cur)
    }
    expanded = next
  }

  async function loadTreeChildren(p) {
    try {
      const list = await webdav.listDirectory(p)
      treeCache[p] = list
        .filter((e) => e.is_dir)
        .map((e) => ({ name: e.name, path: e.path }))
      treeCache = { ...treeCache }
    } catch {
      treeCache[p] = []
      treeCache = { ...treeCache }
    }
  }

  async function toggleExpand(p, ev) {
    ev?.stopPropagation()
    const next = new Set(expanded)
    if (next.has(p)) {
      next.delete(p)
    } else {
      next.add(p)
      if (!treeCache[p]) await loadTreeChildren(p)
    }
    expanded = next
  }

  function treeRows() {
    /** @type {{ path: string, label: string, depth: number, hasChildren: boolean }[]} */
    const rows = []
    function walk(nodePath, label, depth) {
      const kids = treeCache[nodePath] || []
      rows.push({
        path: nodePath,
        label,
        depth,
        hasChildren: kids.length > 0 || !treeCache[nodePath],
      })
      if (expanded.has(nodePath)) {
        for (const c of kids) {
          walk(c.path, c.name, depth + 1)
        }
      }
    }
    for (const r of ROOTS) walk(r.path, r.label, 0)
    return rows
  }

  function openEntry(e) {
    if (e.is_dir) browse(e.path)
    else previewFile(e)
  }

  /** @returns {{ name: string, path: string, is_dir: boolean, size?: number }[]} */
  function getSelectedEntries() {
    return sorted.filter((e) => selectedPaths.has(e.path))
  }

  function syncPreviewWithSelection() {
    const files = getSelectedEntries().filter((e) => !e.is_dir)
    if (files.length === 1) previewFile(files[0])
    else closePreview()
  }

  /** @param {number} from @param {number} to */
  function selectRange(from, to) {
    const lo = Math.min(from, to)
    const hi = Math.max(from, to)
    const next = new Set(selectedPaths)
    for (let i = lo; i <= hi; i++) {
      next.add(sorted[i].path)
    }
    selectedPaths = next
  }

  /** @param {MouseEvent} ev @param {{ name: string, path: string, is_dir: boolean }} entry @param {number} idx */
  function onRowClick(ev, entry, idx) {
    ev.stopPropagation()
    if (ev.shiftKey) {
      const anchor = selectionAnchor ?? idx
      selectRange(anchor, idx)
      if (selectionAnchor === null) selectionAnchor = idx
    } else if (ev.ctrlKey || ev.metaKey) {
      const next = new Set(selectedPaths)
      if (next.has(entry.path)) next.delete(entry.path)
      else next.add(entry.path)
      selectedPaths = next
      selectionAnchor = idx
    } else {
      selectedPaths = new Set([entry.path])
      selectionAnchor = idx
    }
    syncPreviewWithSelection()
  }

  function clearSelection() {
    selectedPaths = new Set()
    selectionAnchor = null
    closePreview()
  }

  /** @param {MouseEvent} ev */
  function onListClick(ev) {
    if (ev.target.closest('tr')) return
    clearSelection()
  }

  /** @param {{ name: string, path: string, is_dir: boolean }[]} files */
  function downloadFiles(files) {
    for (const e of files) {
      window.open(webdav.fileUrl(e.path), '_blank')
    }
  }

  function openFromTree(p) {
    browse(p)
  }

  async function goUp() {
    const parent = parentPath(path)
    if (parent) await browse(parent)
  }

  async function previewFile(e) {
    closePreview()
    const kind = previewKind(e.name)
    preview = { name: e.name, path: e.path, kind: kind || 'unsupported' }
    if (!kind) return

    loadingPreview = true
    previewError = ''
    try {
      const url = webdav.fileUrl(e.path)
      const res = await fetch(url, { headers: { Authorization: `Bearer ${getToken()}` } })
      if (!res.ok) throw new Error('Impossible de charger le fichier')

      if (kind === 'text') {
        previewText = await res.text()
      } else if (kind === 'image' && isHeic(e.name)) {
        const blob = await blobForPreview(res, e.name)
        previewUrl = await heicToPreviewUrl(blob)
      } else {
        const blob = await blobForPreview(res, e.name)
        previewUrl = URL.createObjectURL(blob)
      }
    } catch (err) {
      previewError = err.message || 'Erreur de prévisualisation'
    } finally {
      loadingPreview = false
    }
  }

  function closePreview() {
    if (previewUrl) URL.revokeObjectURL(previewUrl)
    preview = null
    previewUrl = ''
    previewText = ''
    previewError = ''
    loadingPreview = false
  }

  async function downloadPreview() {
    if (!preview) return
    try {
      const res = await fetch(webdav.fileUrl(preview.path), {
        headers: { Authorization: `Bearer ${getToken()}` },
      })
      if (!res.ok) throw new Error('Téléchargement impossible')
      const blob = await res.blob()
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = preview.name
      a.click()
      URL.revokeObjectURL(a.href)
    } catch (e) {
      error = e.message
    }
  }

  function mkdir() {
    mkdirOpen = true
  }

  async function doMkdir(name) {
    mkdirOpen = false
    const dest = path.replace(/\/$/, '') + '/' + name
    try {
      await webdav.mkcol(dest)
      await loadTreeChildren(path)
      await browse(path, { expandTree: false })
    } catch (e) {
      error = e.message
    }
  }

  async function onUploadFiles(ev) {
    const files = ev.target.files
    if (!files?.length) return
    uploading = true
    uploadPct = 0
    uploadLabel = 'Préparation…'
    error = ''
    try {
      await webdav.uploadFiles(path, Array.from(files), (info) => {
        uploadPct = info.overallPercent
        uploadLabel = `${info.index}/${info.total} — ${info.name}`
      })
      await loadTreeChildren(path)
      await browse(path, { expandTree: false })
    } catch (e) {
      error = e.message
    } finally {
      uploading = false
      uploadPct = 0
      uploadLabel = ''
      ev.target.value = ''
    }
  }

  function fmt(size) {
    if (!size) return '—'
    const u = ['o', 'Ko', 'Mo', 'Go']
    let i = 0, n = size
    while (n >= 1024 && i < 3) { n /= 1024; i++ }
    return `${n.toFixed(1)} ${u[i]}`
  }

  function fileType(e) {
    if (e.is_dir) return 'Dossier'
    const ext = e.name.includes('.') ? e.name.split('.').pop().toUpperCase() : ''
    return ext ? `Fichier ${ext}` : 'Fichier'
  }

  function isRoot(p) {
    return ROOTS.some((r) => r.path === p)
  }

  async function copyPath(p) {
    try {
      await navigator.clipboard.writeText(p)
    } catch {
      error = 'Copie du chemin impossible'
    }
  }

  /** @param {{ name: string, path: string, is_dir: boolean }} e */
  function entryMenuItems(e) {
    /** @type {import('../lib/contextMenu.js').ContextMenuItem[]} */
    const items = [
      { label: 'Ouvrir', icon: e.is_dir ? '📂' : '📄', action: () => openEntry(e) },
    ]
    if (!e.is_dir) {
      items.push({ label: 'Aperçu', icon: '👁', action: () => previewFile(e) })
      items.push({ separator: true })
      items.push({
        label: 'Télécharger',
        icon: '⬇',
        action: () => window.open(webdav.fileUrl(e.path), '_blank'),
      })
    }
    items.push({ separator: true })
    items.push({ label: 'Copier le chemin', icon: '📋', action: () => copyPath(e.path) })
    if (!isRoot(e.path)) {
      items.push({ separator: true })
      items.push({ label: 'Renommer', icon: '✏️', action: () => startRename(e) })
      items.push({ label: 'Supprimer', icon: '🗑', danger: true, action: () => startDelete([e]) })
    }
    return items
  }

  /** @param {{ name: string, path: string, is_dir: boolean }[]} selected */
  function selectionMenuItems(selected) {
    if (selected.length === 1) return entryMenuItems(selected[0])
    const files = selected.filter((e) => !e.is_dir)
    const deletable = selected.filter((e) => !isRoot(e.path))
    /** @type {import('../lib/contextMenu.js').ContextMenuItem[]} */
    const items = []
    if (files.length === 1) {
      items.push({ label: 'Aperçu', icon: '👁', action: () => previewFile(files[0]) })
    }
    if (files.length > 0) {
      items.push({
        label: files.length > 1 ? `Télécharger (${files.length})` : 'Télécharger',
        icon: '⬇',
        action: () => downloadFiles(files),
      })
    }
    if (items.length) items.push({ separator: true })
    items.push({
      label: selected.length > 1 ? 'Copier les chemins' : 'Copier le chemin',
      icon: '📋',
      action: () => copyPath(selected.map((e) => e.path).join('\n')),
    })
    if (deletable.length > 0) {
      items.push({ separator: true })
      items.push({
        label: deletable.length > 1 ? `Supprimer (${deletable.length})` : 'Supprimer',
        icon: '🗑',
        danger: true,
        action: () => startDelete(deletable),
      })
    }
    return items
  }

  /** @param {{ path: string, label: string }} node */
  function treeNodeMenuItems(node) {
    const entry = { path: node.path, name: node.label, is_dir: true }
    return entryMenuItems(entry)
  }

  /** @param {MouseEvent} e @param {{ name: string, path: string, is_dir: boolean }} entry @param {number} idx */
  function onEntryContextMenu(e, entry, idx) {
    if (!selectedPaths.has(entry.path)) {
      selectedPaths = new Set([entry.path])
      selectionAnchor = idx
      syncPreviewWithSelection()
    }
    openContextMenu(e, selectionMenuItems(getSelectedEntries()), ctxMenu)
  }

  /** @param {MouseEvent} e @param {{ path: string, label: string }} node */
  function onTreeContextMenu(e, node) {
    openContextMenu(e, treeNodeMenuItems(node), ctxMenu)
  }

  function onPaneContextMenu(e) {
    e.preventDefault()
    if (e.target.closest('.tree-node, .file-table tr, .toolbar, .preview, .ctx-menu')) return
    clearSelection()
    openContextMenu(
      e,
      [
        { label: 'Nouveau dossier', icon: '📁', action: mkdir },
        { label: 'Actualiser', icon: '🔄', action: () => browse(path, { expandTree: false }) },
        { separator: true },
        { label: 'Copier le chemin', icon: '📋', action: () => copyPath(path) },
      ],
      ctxMenu,
    )
  }

  /** @param {{ name: string, path: string, is_dir: boolean }} e */
  function startRename(e) {
    renameEntry = e
    renameOpen = true
  }

  /** @param {{ name: string, path: string, is_dir: boolean } | { name: string, path: string, is_dir: boolean }[]} e */
  function startDelete(e) {
    deleteTargets = Array.isArray(e) ? e : [e]
    deleteEntry = deleteTargets.length === 1 ? deleteTargets[0] : null
    deleteOpen = true
  }

  async function doRename(newName) {
    renameOpen = false
    const e = renameEntry
    renameEntry = null
    if (!e || newName === e.name) return
    const parent = parentPath(e.path) || path
    try {
      const newPath = await webdav.renamePath(e.path, newName)
      if (preview?.path === e.path) closePreview()
      delete treeCache[e.path]
      const next = new Set(expanded)
      next.delete(e.path)
      if (e.is_dir) next.add(newPath)
      expanded = next
      treeCache = { ...treeCache }
      await loadTreeChildren(parent)
      if (e.is_dir) await loadTreeChildren(newPath)
      const dest = path === e.path ? newPath : path
      await browse(dest, { expandTree: false })
    } catch (err) {
      error = err.message
    }
  }

  async function doDelete() {
    deleteOpen = false
    const targets = [...deleteTargets]
    deleteTargets = []
    deleteEntry = null
    if (!targets.length) return
    const parents = new Set()
    try {
      for (const e of targets) {
        if (preview?.path === e.path) closePreview()
        await webdav.deletePath(e.path)
        delete treeCache[e.path]
        parents.add(parentPath(e.path) || '/volumes')
      }
      const next = new Set(expanded)
      for (const e of targets) next.delete(e.path)
      expanded = next
      treeCache = { ...treeCache }
      selectedPaths = new Set()
      selectionAnchor = null
      for (const p of parents) await loadTreeChildren(p)
      const inDeleted = targets.some(
        (e) => path === e.path || path.startsWith(e.path + '/'),
      )
      const dest = inDeleted ? parentPath(targets[0].path) || '/volumes' : path
      await browse(dest, { expandTree: false })
    } catch (err) {
      error = err.message
    }
  }

  let sorted = $derived(
    [...entries].sort((a, b) => {
      if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
      return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
    })
  )
</script>

<div class="explorer" oncontextmenu={onPaneContextMenu}>
  <div class="toolbar">
    <button class="ghost icon" onclick={goUp} disabled={!parentPath(path)} title="Dossier parent">↑</button>
    <code class="path">{path}</code>
    <button class="ghost" onclick={mkdir} disabled={uploading}>+ Dossier</button>
    <label class="upload-btn ghost" class:disabled={uploading}>
      ↑ Fichiers
      <input type="file" multiple hidden disabled={uploading} onchange={onUploadFiles} />
    </label>
    <label class="upload-btn ghost" class:disabled={uploading}>
      📁 Dossier
      <input type="file" webkitdirectory multiple hidden disabled={uploading} onchange={onUploadFiles} />
    </label>
  </div>

  {#if uploading}
    <div class="upload-progress" role="status">
      <div class="upload-head">
        <span>Envoi en cours…</span>
        <span class="upload-meta">{uploadLabel}</span>
      </div>
      <div class="bar-track">
        <div class="bar-fill" style:width="{uploadPct}%"></div>
      </div>
      <span class="pct">{uploadPct.toFixed(0)}%</span>
    </div>
  {/if}

  {#if error}<p class="err">{error}</p>{/if}

  <div class="panes">
    <aside class="tree-pane" aria-label="Arborescence">
      <p class="pane-title">Arborescence</p>
      <ul class="tree">
        {#each treeRows() as node}
          <li style="padding-left: {node.depth * 14 + 4}px">
            <button
              class="tree-node"
              class:active={path === node.path}
              onclick={() => openFromTree(node.path)}
              oncontextmenu={(e) => onTreeContextMenu(e, node)}
            >
              {#if node.hasChildren}
                <span
                  class="twisty"
                  class:open={expanded.has(node.path)}
                  onclick={(e) => toggleExpand(node.path, e)}
                  role="button"
                  tabindex="0"
                  onkeydown={(e) => e.key === 'Enter' && toggleExpand(node.path, e)}
                >▶</span>
              {:else}
                <span class="twisty spacer"></span>
              {/if}
              <span class="ico">{node.depth === 0 ? '📀' : '📁'}</span>
              <span class="tree-label">{node.label}</span>
            </button>
          </li>
        {/each}
      </ul>
    </aside>

    <div class="main-pane">
      <p class="pane-title">{path.split('/').pop() || path}</p>

      <div class="list-wrap" onclick={onListClick}>
        {#if loading}
          <p class="loading">Chargement…</p>
        {:else if sorted.length === 0}
          <p class="muted empty">Ce dossier est vide.</p>
        {:else}
          <table class="file-table">
            <thead>
              <tr>
                <th>Nom</th>
                <th>Taille</th>
                <th>Type</th>
              </tr>
            </thead>
            <tbody>
              {#each sorted as e, i}
                <tr
                  class:sel={selectedPaths.has(e.path)}
                  class:folder={e.is_dir}
                  onclick={(ev) => onRowClick(ev, e, i)}
                  ondblclick={(ev) => {
                    ev.stopPropagation()
                    openEntry(e)
                  }}
                  oncontextmenu={(ev) => onEntryContextMenu(ev, e, i)}
                >
                  <td class="name">
                    <span class="ico">{e.is_dir ? '📁' : '📄'}</span>
                    {e.name}
                  </td>
                  <td class="size">{e.is_dir ? '' : fmt(e.size)}</td>
                  <td class="type">{fileType(e)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>

      {#if preview}
        <div class="preview">
          <div class="prev-head">
            <strong>{preview.name}</strong>
            <div class="prev-actions">
              <button type="button" class="ghost tiny" onclick={downloadPreview}>⬇ Télécharger</button>
              <button type="button" class="ghost tiny" onclick={closePreview} title="Fermer">×</button>
            </div>
          </div>
          {#if loadingPreview}
            <p class="muted">Chargement…</p>
          {:else if previewError}
            <p class="err">{previewError}</p>
          {:else if preview.kind === 'text' && previewText}
            <pre>{previewText}</pre>
          {:else if preview.kind === 'image' && previewUrl}
            <img src={previewUrl} alt={preview.name} class="img-preview" />
          {:else if preview.kind === 'audio' && previewUrl}
            <audio controls src={previewUrl} class="media"></audio>
          {:else if preview.kind === 'video' && previewUrl}
            <!-- svelte-ignore a11y_media_has_caption -->
            <video controls src={previewUrl} class="vid"></video>
            {#if ['mkv', 'avi', 'mov'].includes(preview.name.split('.').pop()?.toLowerCase() || '')}
              <p class="muted">Si la lecture échoue, le navigateur ne supporte peut‑être pas ce conteneur (MKV/AVI/MOV). Utilisez « Télécharger ».</p>
            {/if}
          {:else if preview.kind === 'unsupported'}
            <p class="muted">Aperçu non disponible pour ce type de fichier.</p>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</div>

<PromptModal
  open={mkdirOpen}
  title="Nouveau dossier"
  label="Nom du dossier"
  placeholder="mon-dossier"
  confirmLabel="Créer"
  onconfirm={doMkdir}
  oncancel={() => (mkdirOpen = false)}
/>

<PromptModal
  open={renameOpen}
  title="Renommer"
  label="Nouveau nom"
  initialValue={renameEntry?.name ?? ''}
  confirmLabel="Renommer"
  onconfirm={doRename}
  oncancel={() => {
    renameOpen = false
    renameEntry = null
  }}
/>

<ConfirmModal
  open={deleteOpen}
  title="Supprimer"
  message={deleteTargets.length > 1
    ? `Supprimer définitivement ${deleteTargets.length} éléments ? Cette action est irréversible.`
    : deleteEntry
      ? `Supprimer définitivement « ${deleteEntry.name} » ?${deleteEntry.is_dir ? ' Ce dossier et son contenu seront effacés.' : ''}`
      : ''}
  variant="danger"
  confirmLabel="Supprimer"
  onconfirm={doDelete}
  oncancel={() => {
    deleteOpen = false
    deleteEntry = null
    deleteTargets = []
  }}
/>

<style>
  .explorer {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 280px;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
    flex-wrap: wrap;
    flex-shrink: 0;
  }
  .toolbar .icon { min-width: 36px; font-size: 16px; }
  .path { flex: 1; font-size: 11px; min-width: 100px; overflow: hidden; text-overflow: ellipsis; }
  .upload-btn {
    cursor: pointer;
    padding: 8px 14px;
    border-radius: 6px;
    border: 1px solid var(--bb-border);
  }
  .upload-btn.disabled { opacity: 0.5; pointer-events: none; }
  .upload-progress {
    margin-bottom: 8px;
    padding: 10px 12px;
    border-radius: 8px;
    border: 1px solid var(--bb-border);
    background: var(--bb-panel);
    font-size: 12px;
  }
  .upload-head {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 6px;
  }
  .upload-meta {
    color: var(--bb-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 60%;
    text-align: right;
  }
  .bar-track {
    height: 6px;
    border-radius: 99px;
    background: rgba(0, 0, 0, 0.25);
    overflow: hidden;
  }
  .bar-fill {
    height: 100%;
    background: var(--bb-accent);
    border-radius: 99px;
    transition: width 0.15s ease;
  }
  .pct {
    display: block;
    margin-top: 4px;
    font-size: 10px;
    color: var(--bb-muted);
    text-align: right;
  }
  .panes {
    display: flex;
    flex: 1;
    min-height: 0;
    border: 1px solid var(--bb-border);
    border-radius: 8px;
    overflow: hidden;
  }
  .tree-pane {
    width: 200px;
    min-width: 160px;
    max-width: 280px;
    border-right: 1px solid var(--bb-border);
    background: var(--bb-panel);
    overflow: auto;
    display: flex;
    flex-direction: column;
  }
  .main-pane {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    overflow: hidden;
  }
  .pane-title {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--bb-muted);
    padding: 6px 10px;
    border-bottom: 1px solid var(--bb-border);
    margin: 0;
    flex-shrink: 0;
  }
  .tree {
    list-style: none;
    margin: 0;
    padding: 4px 0;
    flex: 1;
    overflow: auto;
  }
  .tree-node {
    display: flex;
    align-items: center;
    gap: 2px;
    width: 100%;
    text-align: left;
    padding: 4px 6px 4px 0;
    background: transparent;
    border: none;
    color: var(--bb-text);
    font-size: 12px;
    border-radius: 4px;
    cursor: pointer;
  }
  .tree-node:hover { background: rgba(255, 255, 255, 0.06); }
  .tree-node.active { background: rgba(74, 158, 255, 0.2); }
  .twisty {
    display: inline-block;
    width: 14px;
    font-size: 8px;
    color: var(--bb-muted);
    transition: transform 0.15s;
    flex-shrink: 0;
    cursor: pointer;
    user-select: none;
  }
  .twisty.open { transform: rotate(90deg); }
  .twisty.spacer { visibility: hidden; }
  .tree-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .list-wrap {
    flex: 1;
    overflow: auto;
    min-height: 0;
  }
  .loading, .empty { padding: 12px; font-size: 12px; }
  .file-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }
  .file-table th {
    text-align: left;
    padding: 6px 10px;
    font-size: 10px;
    text-transform: uppercase;
    color: var(--bb-muted);
    border-bottom: 1px solid var(--bb-border);
    background: var(--bb-panel);
    position: sticky;
    top: 0;
  }
  .file-table td {
    padding: 6px 10px;
    border-bottom: 1px solid var(--bb-border);
    cursor: default;
  }
  .file-table tr { user-select: none; cursor: default; }
  .file-table tr:hover td { background: rgba(255, 255, 255, 0.04); }
  .file-table tr.sel td { background: rgba(74, 158, 255, 0.22); }
  .file-table tr.sel:hover td { background: rgba(74, 158, 255, 0.28); }
  .file-table tr.folder { cursor: pointer; }
  .file-table .name { display: flex; align-items: center; gap: 6px; }
  .file-table .size, .file-table .type { color: var(--bb-muted); white-space: nowrap; }
  .file-table .size { width: 72px; }
  .file-table .type { width: 100px; }
  .ico { flex-shrink: 0; }
  .preview {
    flex-shrink: 0;
    max-height: 40%;
    border-top: 1px solid var(--bb-border);
    padding: 10px;
    overflow: auto;
    background: var(--bb-panel);
  }
  .prev-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; margin-bottom: 8px; font-size: 12px; }
  .prev-actions { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
  .prev-actions .tiny { padding: 4px 10px; font-size: 11px; }
  pre { font-size: 11px; white-space: pre-wrap; word-break: break-word; margin: 0; }
  .img-preview { max-width: 100%; max-height: 280px; object-fit: contain; display: block; }
  .vid, .media { max-width: 100%; width: 100%; }
  .vid { max-height: 280px; }
  .muted { color: var(--bb-muted); font-size: 11px; }
  .err { color: var(--bb-danger); font-size: 12px; margin-bottom: 6px; }
</style>
