<script>
  import { onMount } from 'svelte'
  import * as webdav from './webdav.js'

  const ROOTS = [
    { path: '/volumes', label: 'Volumes' },
    { path: '/data', label: 'Système' },
  ]

  /** @type {{ path?: string, height?: string }} */
  let { path = $bindable('/volumes'), height = '220px' } = $props()

  let browsePath = $state('/volumes')
  let entries = $state([])
  let loading = $state(false)
  let treeError = $state('')

  /** @type {Record<string, { name: string, path: string }[]>} */
  let treeCache = $state({})
  /** @type {Set<string>} */
  let expanded = $state(new Set(['/volumes', '/data']))

  onMount(() => {
    ROOTS.forEach((r) => loadTreeChildren(r.path))
    browse(browsePath)
  })

  function parentPath(p) {
    if (ROOTS.some((r) => r.path === p)) return null
    const i = p.lastIndexOf('/')
    if (i <= 0) return '/'
    return p.slice(0, i) || '/'
  }

  async function browse(p) {
    loading = true
    treeError = ''
    try {
      const list = await webdav.listDirectory(p)
      entries = list.filter((e) => e.is_dir)
      browsePath = p
      path = p
      await ensureExpandedTo(p)
    } catch (e) {
      treeError = e.message
      entries = []
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
        hasChildren: kids.length > 0 || treeCache[nodePath] === undefined,
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

  function selectFolder(p) {
    path = p
    browse(p)
  }
</script>

<div class="path-browser" style:--pb-height={height}>
  <div class="selected">
    <span class="lbl">Chemin sélectionné</span>
    <code>{path}</code>
  </div>

  {#if treeError}<p class="err">{treeError}</p>{/if}

  <div class="panes">
    <aside class="tree-pane" aria-label="Arborescence">
      <ul class="tree">
        {#each treeRows() as node}
          <li style="padding-left: {node.depth * 12 + 4}px">
            <button
              type="button"
              class="tree-node"
              class:active={path === node.path}
              class:here={browsePath === node.path}
              onclick={() => selectFolder(node.path)}
            >
              {#if node.hasChildren}
                <span
                  class="twisty"
                  class:open={expanded.has(node.path)}
                  onclick={(e) => toggleExpand(node.path, e)}
                  onkeydown={(e) => e.key === 'Enter' && toggleExpand(node.path, e)}
                  role="button"
                  tabindex="0"
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

    <div class="list-pane">
      <div class="list-head">
        <button type="button" class="ghost tiny" onclick={() => { const p = parentPath(browsePath); if (p) browse(p) }} disabled={!parentPath(browsePath)} title="Parent">↑</button>
        <span class="cur">{browsePath.split('/').pop() || browsePath}</span>
        <button type="button" class="ghost tiny select-here" onclick={() => (path = browsePath)}>Sélectionner</button>
      </div>
      {#if loading}
        <p class="muted load">Chargement…</p>
      {:else if entries.length === 0}
        <p class="muted load">Aucun sous-dossier.</p>
      {:else}
        <ul class="folders">
          {#each entries as e}
            <li>
              <button type="button" class="folder-row" class:sel={path === e.path} onclick={() => selectFolder(e.path)}>
                <span>📁</span>
                <span>{e.name}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>

  <label class="manual">
    Chemin (manuel)
    <input type="text" bind:value={path} placeholder="/volumes/mon-volume" />
  </label>
</div>

<style>
  .path-browser {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 100%;
  }
  .selected {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .selected .lbl {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--bb-muted);
  }
  .selected code {
    font-size: 11px;
    padding: 6px 8px;
    background: rgba(0, 0, 0, 0.2);
    border-radius: 4px;
    border: 1px solid var(--bb-border);
    word-break: break-all;
  }
  .panes {
    display: flex;
    height: var(--pb-height, 220px);
    min-height: 160px;
    border: 1px solid var(--bb-border);
    border-radius: 8px;
    overflow: hidden;
    background: rgba(0, 0, 0, 0.15);
  }
  .tree-pane {
    width: 42%;
    min-width: 130px;
    border-right: 1px solid var(--bb-border);
    overflow: auto;
    padding: 4px 0;
  }
  .tree { list-style: none; margin: 0; padding: 0; }
  .tree-node {
    display: flex;
    align-items: center;
    gap: 4px;
    width: 100%;
    text-align: left;
    padding: 3px 6px 3px 2px;
    border: none;
    background: transparent;
    color: var(--bb-text);
    font-size: 11px;
    border-radius: 4px;
    cursor: pointer;
  }
  .tree-node:hover { background: rgba(255, 255, 255, 0.06); }
  .tree-node.active { background: rgba(61, 155, 233, 0.25); font-weight: 600; }
  .tree-node.here:not(.active) { background: rgba(255, 255, 255, 0.04); }
  .twisty {
    width: 14px;
    flex-shrink: 0;
    font-size: 8px;
    color: var(--bb-muted);
    transition: transform 0.15s;
    cursor: pointer;
  }
  .twisty.open { transform: rotate(90deg); }
  .twisty.spacer { visibility: hidden; }
  .ico { flex-shrink: 0; font-size: 12px; }
  .tree-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .list-pane {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    overflow: hidden;
  }
  .list-head {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 8px;
    border-bottom: 1px solid var(--bb-border);
    font-size: 11px;
    flex-shrink: 0;
  }
  .cur {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 600;
  }
  .tiny { padding: 2px 8px; font-size: 10px; }
  .select-here { margin-left: auto; }
  .folders {
    list-style: none;
    margin: 0;
    padding: 4px;
    overflow: auto;
    flex: 1;
  }
  .folder-row {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    text-align: left;
    padding: 5px 8px;
    border: none;
    background: transparent;
    color: var(--bb-text);
    font-size: 11px;
    border-radius: 4px;
    cursor: pointer;
  }
  .folder-row:hover { background: rgba(255, 255, 255, 0.06); }
  .folder-row.sel { background: rgba(61, 155, 233, 0.2); }
  .load { padding: 12px; font-size: 11px; }
  .muted { color: var(--bb-muted); }
  .manual {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 11px;
    color: var(--bb-muted);
  }
  .manual input { font-size: 12px; }
  .err { color: var(--bb-danger); font-size: 11px; margin: 0; }
</style>
