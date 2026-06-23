<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { poll } from '../lib/poll.js'

  const MAX_LINES = 2500
  const POLL_MS = 3000
  const HIGHLIGHT_MS = 2800

  let fetching = false

  const SOURCE_COLORS = {
    'bytebay-panel': '#6eb5ff',
    'bytebay-agent': '#3dd68c',
    'bytebay-web': '#c792ea',
    'bytebay-engine': '#f0c674',
    'bytebay-engine-proc': '#ffb86c',
    kernel: '#ff6b6b',
    system: '#8ba3c0',
  }

  let sources = $state([])
  let enabled = $state({})
  let lines = $state([])
  let lastSince = $state('')
  let loading = $state(true)
  let error = $state('')
  let autoscroll = $state(true)
  let logEl = $state(null)
  /** @type {Set<string>} */
  let highlighted = $state(new Set())

  onMount(() => {
    init()
    const stop = poll(tick, POLL_MS)
    return stop
  })

  function entryKey(line) {
    return `${line.time}\0${line.source}\0${line.line}`
  }

  function sortEntries(entries) {
    return [...entries].sort((a, b) => {
      const ta = Date.parse(a.time || '') || 0
      const tb = Date.parse(b.time || '') || 0
      if (ta !== tb) return ta - tb
      const sa = a.source || ''
      const sb = b.source || ''
      if (sa !== sb) return sa.localeCompare(sb)
      return (a.line || '').localeCompare(b.line || '')
    })
  }

  function mergeEntries(existing, batch) {
    if (!existing.length) return sortEntries(batch)
    const seen = new Set(existing.map(entryKey))
    const added = []
    for (const line of batch) {
      const key = entryKey(line)
      if (seen.has(key)) continue
      seen.add(key)
      added.push(line)
    }
    if (!added.length) return existing
    return sortEntries([...existing, ...added])
  }

  function cursorFromEntries(entries) {
    let best = ''
    let bestMs = 0
    for (const e of entries) {
      const ms = Date.parse(e.time || '')
      if (ms >= bestMs) {
        bestMs = ms
        best = e.time
      }
    }
    return best
  }

  function markNew(batch) {
    if (!batch.length) return
    const keys = batch.map(entryKey)
    highlighted = new Set([...highlighted, ...keys])
    const added = new Set(keys)
    setTimeout(() => {
      highlighted = new Set([...highlighted].filter((k) => !added.has(k)))
    }, HIGHLIGHT_MS)
  }

  function scrollToBottom() {
    if (!autoscroll || !logEl) return
    requestAnimationFrame(() => {
      if (logEl) logEl.scrollTop = logEl.scrollHeight
    })
  }

  async function init() {
    loading = true
    error = ''
    try {
      sources = await api.logSources()
      const next = { ...enabled }
      for (const s of sources) {
        if (next[s.id] === undefined) next[s.id] = true
      }
      enabled = next
      lines = []
      lastSince = ''
      highlighted = new Set()
      await tick(true)
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  function selectedCsv() {
    return sources
      .filter((s) => enabled[s.id])
      .map((s) => s.id)
      .join(',')
  }

  async function tick(reset = false) {
    if (fetching) return
    if (loading && !reset) return
    const csv = selectedCsv()
    if (!csv) return
    fetching = true
    try {
      const res = await api.logs(lastSince, csv)
      const batch = res.entries || []
      if (reset || !lastSince) {
        lines = sortEntries(batch)
      } else if (batch.length) {
        const prevKeys = new Set(lines.map(entryKey))
        lines = mergeEntries(lines, batch)
        if (lines.length > MAX_LINES) lines = lines.slice(-MAX_LINES)
        markNew(batch.filter((l) => !prevKeys.has(entryKey(l))))
      }
      if (lines.length) lastSince = cursorFromEntries(lines)
      else if (reset || !lastSince) lastSince = ''
      if (batch.length) scrollToBottom()
      error = ''
    } catch (e) {
      error = e.message
    } finally {
      fetching = false
    }
  }

  function toggleSource(id) {
    enabled = { ...enabled, [id]: !enabled[id] }
    lines = []
    lastSince = ''
    highlighted = new Set()
    tick(true)
  }

  function toggleAll(on) {
    const next = { ...enabled }
    for (const s of sources) next[s.id] = on
    enabled = next
    lines = []
    lastSince = ''
    highlighted = new Set()
    tick(true)
  }

  function colorFor(source) {
    return SOURCE_COLORS[source] || 'var(--bb-muted)'
  }

  function labelFor(id) {
    return sources.find((s) => s.id === id)?.label || id
  }

  function formatTime(ts) {
    if (!ts) return ''
    try {
      return new Date(ts).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    } catch {
      return ts.slice(11, 19)
    }
  }

  let visibleLines = $derived(lines.filter((l) => enabled[l.source] !== false))

  $effect(() => {
    visibleLines.length
    scrollToBottom()
  })
</script>

<div class="logs-app">
<div class="toolbar">
  <span class="lbl">Sources</span>
  <button class="ghost tiny" onclick={() => toggleAll(true)}>Tout</button>
  <button class="ghost tiny" onclick={() => toggleAll(false)}>Aucun</button>
  <label class="chk"><input type="checkbox" bind:checked={autoscroll} /> Défilement auto</label>
  <button class="ghost tiny" onclick={() => { lines = []; lastSince = ''; highlighted = new Set(); tick(true) }}>Effacer</button>
</div>

<div class="filters">
  {#each sources as s}
    <label class="filter" style:--c={colorFor(s.id)}>
      <input type="checkbox" checked={enabled[s.id]} onchange={() => toggleSource(s.id)} />
      <span>{s.label}</span>
    </label>
  {/each}
</div>

{#if loading && !lines.length}
  <p class="muted">Chargement des journaux…</p>
{:else}
  <div class="log-view" bind:this={logEl}>
    {#each visibleLines as line, i (entryKey(line) + '\0' + i)}
      <div class="row" class:new={highlighted.has(entryKey(line))}>
        <span class="ts">{formatTime(line.time)}</span>
        <span class="src" style:color={colorFor(line.source)} title={labelFor(line.source)}>
          {line.source}
        </span>
        <span class="msg">{line.line}</span>
      </div>
    {/each}
    {#if !visibleLines.length}
      <p class="muted empty">Aucune entrée pour les sources sélectionnées.</p>
    {/if}
  </div>
{/if}

{#if error}<p class="err">{error}</p>{/if}
</div>

<style>
  .logs-app {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    margin-bottom: 8px;
    font-size: 12px;
  }
  .lbl { color: var(--bb-muted); font-weight: 600; }
  .tiny { padding: 2px 8px; font-size: 11px; }
  .chk { flex-direction: row; align-items: center; gap: 6px; color: var(--bb-text); font-size: 11px; }
  .chk input { width: auto; }
  .filters {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 10px;
    margin-bottom: 10px;
    padding-bottom: 8px;
    border-bottom: 1px solid var(--bb-border);
  }
  .filter {
    display: flex;
    align-items: center;
    gap: 5px;
    font-size: 11px;
    color: var(--bb-text);
    border-left: 3px solid var(--c, var(--bb-border));
    padding-left: 6px;
  }
  .filter input { width: auto; }
  .log-view {
    flex: 1;
    min-height: 200px;
    overflow: auto;
    font-family: ui-monospace, 'Cascadia Code', 'Source Code Pro', monospace;
    font-size: 11px;
    line-height: 1.45;
    background: rgba(0, 0, 0, 0.25);
    border: 1px solid var(--bb-border);
    border-radius: 6px;
    padding: 8px;
    display: flex;
    flex-direction: column;
  }
  .row {
    display: grid;
    grid-template-columns: 64px 120px 1fr;
    gap: 8px;
    padding: 2px 4px;
    margin: 0 -4px;
    border-radius: 4px;
  }
  .row.new {
    animation: log-appear 2.8s ease-out;
  }
  .row.new .msg {
    animation: log-text-glow 2.8s ease-out;
  }
  @keyframes log-appear {
    0% {
      background: rgba(61, 155, 233, 0.42);
      box-shadow: inset 0 0 0 1px rgba(110, 181, 255, 0.55), 0 0 14px rgba(61, 155, 233, 0.35);
    }
    35% {
      background: rgba(61, 155, 233, 0.22);
      box-shadow: inset 0 0 0 1px rgba(110, 181, 255, 0.3), 0 0 8px rgba(61, 155, 233, 0.2);
    }
    100% {
      background: transparent;
      box-shadow: none;
    }
  }
  @keyframes log-text-glow {
    0% { color: #e8f4ff; text-shadow: 0 0 8px rgba(110, 181, 255, 0.9); }
    40% { color: #d4eaff; text-shadow: 0 0 4px rgba(110, 181, 255, 0.45); }
    100% { color: var(--bb-text); text-shadow: none; }
  }
  .ts { color: var(--bb-muted); white-space: nowrap; }
  .src { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-weight: 600; }
  .msg { white-space: pre-wrap; word-break: break-word; color: var(--bb-text); }
  .muted { color: var(--bb-muted); font-size: 12px; }
  .empty { padding: 12px; }
  .err { color: var(--bb-danger); margin-top: 8px; font-size: 12px; }
</style>
