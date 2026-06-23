<script>
  import { clampWindowPosition, clampWindowBounds } from './windowPlacement.js'

  let {
    title,
    children,
    onclose,
    x = 80,
    y = 60,
    width = 640,
    height = 420,
    z = 1,
    flash = false,
    onfocus,
    onmove,
    onresize,
  } = $props()

  let dragging = $state(false)
  let resizing = $state(null)
  let dx = 0
  let dy = 0
  let resizeStart = null
  let captureEl = $state(null)

  /** Position/taille locales pendant drag ou resize — null = utiliser les props parent. */
  let liveX = $state(null)
  let liveY = $state(null)
  let liveW = $state(null)
  let liveH = $state(null)

  let posX = $derived(liveX ?? x)
  let posY = $derived(liveY ?? y)
  let sizeW = $derived(liveW ?? width)
  let sizeH = $derived(liveH ?? height)

  function startDrag(e) {
    if (e.button !== 0) return
    if (e.target.closest('.close, .resize-handle')) return
    const workspace = e.currentTarget.closest('.workspace')
    const rect = workspace?.getBoundingClientRect()
    if (!rect) return
    dragging = true
    liveX = posX
    liveY = posY
    dx = e.clientX - rect.left - posX
    dy = e.clientY - rect.top - posY
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
    const rect = captureEl?.closest('.workspace')?.getBoundingClientRect()
    if (!rect) return
    const rawX = e.clientX - rect.left - dx
    const rawY = e.clientY - rect.top - dy
    const c = clampWindowPosition(rawX, rawY, sizeW, sizeH)
    liveX = c.x
    liveY = c.y
  }

  function endDrag(e) {
    if (!dragging) return
    dragging = false
    releaseCapture(e)
    if (liveX != null && liveY != null) {
      onmove?.({ x: liveX, y: liveY })
    }
    liveX = null
    liveY = null
  }

  function startResize(e, mode) {
    if (e.button !== 0) return
    e.stopPropagation()
    e.preventDefault()
    onfocus?.()
    resizing = mode
    resizeStart = {
      px: e.clientX,
      py: e.clientY,
      x: posX,
      y: posY,
      w: sizeW,
      h: sizeH,
    }
    liveX = posX
    liveY = posY
    liveW = sizeW
    liveH = sizeH
    captureEl = e.currentTarget
    captureEl.setPointerCapture(e.pointerId)
    window.addEventListener('pointermove', onResizeMove)
    window.addEventListener('pointerup', endResize)
    window.addEventListener('pointercancel', endResize)
  }

  function onResizeMove(e) {
    if (!resizing || !resizeStart) return
    e.preventDefault()
    const ddx = e.clientX - resizeStart.px
    const ddy = e.clientY - resizeStart.py
    let nx = resizeStart.x
    let ny = resizeStart.y
    let nw = resizeStart.w
    let nh = resizeStart.h

    if (resizing.includes('e')) nw = resizeStart.w + ddx
    if (resizing.includes('w')) {
      nw = resizeStart.w - ddx
      nx = resizeStart.x + ddx
    }
    if (resizing.includes('s')) nh = resizeStart.h + ddy
    if (resizing.includes('n')) {
      nh = resizeStart.h - ddy
      ny = resizeStart.y + ddy
    }

    const c = clampWindowBounds(nx, ny, nw, nh)
    liveX = c.x
    liveY = c.y
    liveW = c.w
    liveH = c.h
  }

  function endResize(e) {
    if (!resizing) return
    resizing = null
    resizeStart = null
    releaseCapture(e)
    if (liveX != null && liveY != null && liveW != null && liveH != null) {
      onresize?.({ x: liveX, y: liveY, w: liveW, h: liveH })
    }
    liveX = null
    liveY = null
    liveW = null
    liveH = null
  }

  function releaseCapture(e) {
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
    window.removeEventListener('pointermove', onResizeMove)
    window.removeEventListener('pointerup', endResize)
    window.removeEventListener('pointercancel', endResize)
  }

  function handleClose(e) {
    e.stopPropagation()
    e.preventDefault()
    onclose?.()
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="window"
  class:flash
  style="left:{posX}px;top:{posY}px;width:{sizeW}px;height:{sizeH}px;z-index:{z}"
>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="titlebar" onpointerdown={startDrag}>
    <span class="title">{title}</span>
    <button
      type="button"
      class="close"
      onclick={handleClose}
      onpointerdown={(e) => e.stopPropagation()}
      onmousedown={(e) => e.stopPropagation()}
      aria-label="Fermer"
    >×</button>
  </div>
  <div class="content">
    {@render children()}
  </div>

  <!-- Poignées de redimensionnement -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="resize-handle n" onpointerdown={(e) => startResize(e, 'n')}></div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="resize-handle s" onpointerdown={(e) => startResize(e, 's')}></div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="resize-handle e" onpointerdown={(e) => startResize(e, 'e')}></div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="resize-handle w" onpointerdown={(e) => startResize(e, 'w')}></div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="resize-handle ne" onpointerdown={(e) => startResize(e, 'ne')}></div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="resize-handle nw" onpointerdown={(e) => startResize(e, 'nw')}></div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="resize-handle se" onpointerdown={(e) => startResize(e, 'se')}></div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="resize-handle sw" onpointerdown={(e) => startResize(e, 'sw')}></div>
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
  .window.flash {
    animation: window-flash 0.55s ease;
  }
  @keyframes window-flash {
    0%, 100% {
      box-shadow: var(--bb-shadow);
      border-color: var(--bb-border);
    }
    35% {
      box-shadow: 0 0 0 2px rgba(61, 155, 233, 0.85), 0 0 28px rgba(61, 155, 233, 0.45);
      border-color: rgba(61, 155, 233, 0.9);
    }
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
    cursor: pointer;
    flex-shrink: 0;
  }
  .close:hover { color: var(--bb-danger); background: rgba(231,76,92,0.15); }
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px;
    min-height: 0;
  }
  .resize-handle {
    position: absolute;
    touch-action: none;
    z-index: 2;
  }
  .n, .s { left: 8px; right: 8px; height: 6px; }
  .n { top: 0; cursor: n-resize; }
  .s { bottom: 0; cursor: s-resize; }
  .e, .w { top: 8px; bottom: 8px; width: 6px; }
  .e { right: 0; cursor: e-resize; }
  .w { left: 0; cursor: w-resize; }
  .ne, .nw, .se, .sw { width: 14px; height: 14px; }
  .ne { top: 0; right: 0; cursor: ne-resize; }
  .nw { top: 0; left: 0; cursor: nw-resize; }
  .se { bottom: 0; right: 0; cursor: se-resize; }
  .sw { bottom: 0; left: 0; cursor: sw-resize; }
</style>
