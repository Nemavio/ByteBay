<script>
  /** @type {{ open: boolean, x: number, y: number, items: import('./contextMenu.js').ContextMenuItem[], onclose: () => void }} */
  let { open, x, y, items = [], onclose } = $props()

  let menuEl = $state(null)
  let pos = $state({ x: 0, y: 0 })

  $effect(() => {
    if (!open) return
    const pad = 8
    let left = x
    let top = y
    requestAnimationFrame(() => {
      if (!menuEl) return
      const r = menuEl.getBoundingClientRect()
      if (left + r.width > window.innerWidth - pad) left = Math.max(pad, window.innerWidth - r.width - pad)
      if (top + r.height > window.innerHeight - pad) top = Math.max(pad, window.innerHeight - r.height - pad)
      pos = { x: left, y: top }
    })
    pos = { x: left, y: top }

    const onPointer = (e) => {
      if (menuEl?.contains(e.target)) return
      onclose()
    }
    const onKey = (e) => {
      if (e.key === 'Escape') {
        e.stopImmediatePropagation()
        onclose()
      }
    }
    window.addEventListener('pointerdown', onPointer, true)
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('pointerdown', onPointer, true)
      window.removeEventListener('keydown', onKey)
    }
  })

  function pick(item) {
    if (item.disabled || item.separator) return
    onclose()
    item.action?.()
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="ctx-menu"
    bind:this={menuEl}
    style:left="{pos.x}px"
    style:top="{pos.y}px"
    role="menu"
    oncontextmenu={(e) => e.preventDefault()}
  >
    {#each items as item}
      {#if item.separator}
        <div class="sep" role="separator"></div>
      {:else}
        <button
          type="button"
          class="item"
          class:danger={item.danger}
          disabled={item.disabled}
          role="menuitem"
          onclick={() => pick(item)}
        >
          {#if item.icon}<span class="ico" aria-hidden="true">{item.icon}</span>{/if}
          <span>{item.label}</span>
        </button>
      {/if}
    {/each}
  </div>
{/if}

<style>
  .ctx-menu {
    position: fixed;
    z-index: 30000;
    min-width: 180px;
    max-width: min(280px, calc(100vw - 16px));
    padding: 4px;
    background: var(--bb-panel);
    border: 1px solid var(--bb-border);
    border-radius: 8px;
    box-shadow: var(--bb-shadow);
    backdrop-filter: blur(8px);
  }
  .item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    text-align: left;
    padding: 7px 10px;
    border: none;
    border-radius: 5px;
    background: transparent;
    color: var(--bb-text);
    font-size: 12px;
    cursor: pointer;
  }
  .item:hover:not(:disabled) { background: rgba(255, 255, 255, 0.08); }
  .item:disabled { opacity: 0.45; cursor: not-allowed; }
  .item.danger { color: var(--bb-danger); }
  .item.danger:hover:not(:disabled) { background: rgba(231, 76, 92, 0.12); }
  .ico { width: 16px; text-align: center; flex-shrink: 0; }
  .sep {
    height: 1px;
    margin: 4px 6px;
    background: var(--bb-border);
  }
</style>
