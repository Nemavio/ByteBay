<script>
  let { title, children, onclose, x = 80, y = 60, width = 640, height = 420, z = 1, onfocus } = $props()

  let dragging = $state(false)
  let dx = 0
  let dy = 0
  let captureEl = null
  let posX = $state(x)
  let posY = $state(y)

  function startDrag(e) {
    if (e.target.closest('.close')) return
    dragging = true
    dx = e.clientX - posX
    dy = e.clientY - posY
    onfocus?.()
    captureEl = e.currentTarget
    captureEl.setPointerCapture(e.pointerId)
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', endDrag)
    window.addEventListener('pointercancel', endDrag)
  }

  function onMove(e) {
    if (!dragging) return
    e.preventDefault()
    posX = Math.max(0, e.clientX - dx)
    posY = Math.max(44, e.clientY - dy)
  }

  function endDrag(e) {
    if (!dragging) return
    dragging = false
    if (captureEl) {
      try {
        captureEl.releasePointerCapture(e.pointerId)
      } catch {
        /* pointer already released */
      }
      captureEl = null
    }
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', endDrag)
    window.removeEventListener('pointercancel', endDrag)
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="window"
  style="left:{posX}px;top:{posY}px;width:{width}px;height:{height}px;z-index:{z}"
  onclick={() => onfocus?.()}
>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="titlebar" onpointerdown={startDrag}>
    <span class="title">{title}</span>
    <button class="close" onclick={onclose} onpointerdown={(e) => e.stopPropagation()} aria-label="Fermer">×</button>
  </div>
  <div class="content">
    {@render children()}
  </div>
</div>

<style>
  .window {
    position: absolute;
    background: var(--bb-window);
    border: 1px solid var(--bb-border);
    border-radius: 10px;
    box-shadow: var(--bb-shadow);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    background: var(--bb-titlebar);
    cursor: move;
    touch-action: none;
    user-select: none;
    border-bottom: 1px solid var(--bb-border);
  }
  .title { font-weight: 600; font-size: 13px; }
  .close {
    width: 28px;
    height: 28px;
    padding: 0;
    line-height: 1;
    font-size: 18px;
    background: transparent;
    color: var(--bb-muted);
  }
  .close:hover { color: var(--bb-danger); background: rgba(231,76,92,0.15); }
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px;
  }
</style>
