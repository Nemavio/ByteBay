<script>
  import { onMount } from 'svelte'
  import { api, getToken } from '../lib/api.js'

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

  /** @type {Record<string, { name: string, path: string }[]>} */
  let treeCache = $state({})
  /** @type {Set<string>} */
  let expanded = $state(new Set(['/volumes', '/data']))

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
    try {
      const list = await api.files(p)
      entries = list.filter((e) => e.name !== '..')
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
      const list = await api.files(p)
      treeCache[p] = list
        .filter((e) => e.is_dir && e.name !== '..')
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

  function openFromTree(p) {
    browse(p)
  }

  async function goUp() {
    const parent = parentPath(path)
    if (parent) await browse(parent)
  }

  async function previewFile(e) {
    closePreview()
    const url = api.fileUrl(e.path)
    const ext = e.name.split('.').pop()?.toLowerCase() || ''
    preview = { name: e.name, path: e.path, type: ext }

    if (['txt', 'md', 'log', 'json', 'csv', 'xml', 'html', 'css', 'js'].includes(ext)) {
      const res = await fetch(url, { headers: { Authorization: `Bearer ${getToken()}` } })
      previewText = await res.text()
    } else if (['mp3', 'ogg', 'wav', 'm4a', 'mp4', 'webm', 'mkv'].includes(ext)) {
      const res = await fetch(url, { headers: { Authorization: `Bearer ${getToken()}` } })
      previewUrl = URL.createObjectURL(await res.blob())
    }
  }

  function closePreview() {
    if (previewUrl) URL.revokeObjectURL(previewUrl)
    preview = null
    previewUrl = ''
    previewText = ''
  }

  async function mkdir() {
    const name = prompt('Nom du dossier :')
    if (!name) return
    await api.filesMkdir(path.replace(/\/$/, '') + '/' + name)
    await loadTreeChildren(path)
    await browse(path, { expandTree: false })
  }

  async function onUpload(ev) {
    const files = ev.target.files
    if (!files?.length) return
    for (const f of files) {
      await api.filesUpload(path, f)
    }
    ev.target.value = ''
    await browse(path, { expandTree: false })
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

  let sorted = $derived(
    [...entries].sort((a, b) => {
      if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
      return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
    })
  )
</script>

<div class="explorer">
  <div class="toolbar">
    <button class="ghost icon" onclick={goUp} disabled={!parentPath(path)} title="Dossier parent">↑</button>
    <code class="path">{path}</code>
    <button class="ghost" onclick={mkdir}>+ Dossier</button>
    <label class="upload-btn ghost">
      ↑ Upload
      <input type="file" multiple hidden onchange={onUpload} />
    </label>
  </div>

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

      <div class="list-wrap">
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
              {#each sorted as e}
                <tr
                  class:sel={preview?.path === e.path}
                  class:folder={e.is_dir}
                  onclick={() => openEntry(e)}
                  ondblclick={() => e.is_dir && browse(e.path)}
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
            <button class="ghost" onclick={closePreview}>×</button>
          </div>
          {#if previewText}
            <pre>{previewText}</pre>
          {:else if previewUrl && ['mp3', 'ogg', 'wav', 'm4a'].includes(preview.type)}
            <audio controls src={previewUrl}></audio>
          {:else if previewUrl}
            <!-- svelte-ignore a11y_media_has_caption -->
            <video controls src={previewUrl} class="vid"></video>
          {:else}
            <p class="muted">Aperçu non disponible — <a href={api.fileUrl(preview.path)} target="_blank">Télécharger</a></p>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</div>

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
  .upload-btn { cursor: pointer; padding: 8px 14px; border-radius: 6px; border: 1px solid var(--bb-border); }
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
  .file-table tr:hover td { background: rgba(255, 255, 255, 0.04); }
  .file-table tr.sel td { background: rgba(74, 158, 255, 0.15); }
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
  .prev-head { display: flex; justify-content: space-between; margin-bottom: 8px; font-size: 12px; }
  pre { font-size: 11px; white-space: pre-wrap; word-break: break-word; margin: 0; }
  .vid { max-width: 100%; max-height: 200px; }
  .muted { color: var(--bb-muted); font-size: 11px; }
  .err { color: var(--bb-danger); font-size: 12px; margin-bottom: 6px; }
</style>
