const GRID = 24
const PAD = 16
const TASKBAR_H = 44

/** Grille invisible des raccourcis bureau (haut → bas, puis gauche → droite). */
export const ICON_CELL_W = 88
export const ICON_CELL_H = 96
export const ICON_PAD = 16

function snap(value, grid = GRID) {
  return Math.round(value / grid) * grid
}

/** Taille utile du bureau (sous la barre des tâches). */
export function workspaceSize() {
  return {
    width: window.innerWidth,
    height: Math.max(200, window.innerHeight - TASKBAR_H),
  }
}

/** Position grille pour une nouvelle fenêtre, sans dépasser l'écran. */
export function nextWindowPosition(index, width, height) {
  const { width: wsW, height: wsH } = workspaceSize()
  const maxX = Math.max(PAD, wsW - width - PAD)
  const maxY = Math.max(PAD, wsH - height - PAD)
  const cols = Math.max(1, Math.floor((wsW - PAD * 2) / (GRID * 3)))
  const col = index % cols
  const row = Math.floor(index / cols)
  const x = snap(Math.min(PAD + col * GRID * 3, maxX))
  const y = snap(Math.min(PAD + row * GRID * 3, maxY))
  return { x, y }
}

/** Borne une fenêtre dans le bureau (sans accrochage grille). */
export function clampWindowPosition(x, y, width, height) {
  const { width: wsW, height: wsH } = workspaceSize()
  const maxX = Math.max(PAD, wsW - width - PAD)
  const maxY = Math.max(PAD, wsH - height - PAD)
  return {
    x: Math.min(Math.max(PAD, x), maxX),
    y: Math.min(Math.max(PAD, y), maxY),
  }
}

/** Borne la taille et la position lors d'un redimensionnement. */
export function clampWindowBounds(x, y, width, height) {
  const { width: wsW, height: wsH } = workspaceSize()
  const minW = 280
  const minH = 160
  let w = Math.max(minW, width)
  let h = Math.max(minH, height)
  let nx = x
  let ny = y
  w = Math.min(w, wsW - PAD * 2)
  h = Math.min(h, wsH - PAD * 2)
  const pos = clampWindowPosition(nx, ny, w, h)
  nx = pos.x
  ny = pos.y
  if (nx + w > wsW - PAD) w = wsW - PAD - nx
  if (ny + h > wsH - PAD) h = wsH - PAD - ny
  return { x: nx, y: ny, w: Math.max(minW, w), h: Math.max(minH, h) }
}

const CHILD_OVERLAP = 48

/**
 * Place une fenêtre enfant à droite du parent, légèrement superposée.
 * @param {{ x: number, y: number, w: number, h: number }} parent
 * @param {number} width
 * @param {number} height
 * @param {Array<{ x: number, y: number, custom?: boolean }>} [existing]
 */
export function childWindowPosition(parent, width, height, existing = []) {
  const { width: wsW, height: wsH } = workspaceSize()
  const cascade = existing.filter((w) => w.custom && Math.abs(w.x - parent.x) < parent.w).length

  let x = parent.x + parent.w - CHILD_OVERLAP + cascade * (GRID / 2)
  let y = parent.y + cascade * (GRID / 2)

  // Pas assez de place à droite → à gauche du parent
  if (x + width > wsW - PAD) {
    x = parent.x - width + CHILD_OVERLAP - cascade * (GRID / 2)
  }
  // Toujours hors cadre → sous le parent
  if (x < PAD || x + width > wsW - PAD) {
    x = parent.x + cascade * (GRID / 2)
    y = parent.y + parent.h - CHILD_OVERLAP + cascade * (GRID / 2)
  }

  return clampWindowPosition(x, y, width, height)
}

/**
 * Positions des icônes bureau sur une grille invisible.
 * Remplissage : colonne du haut vers le bas, puis colonne suivante à droite.
 * @param {number} count
 */
export function desktopIconPositions(count) {
  const { width: wsW, height: wsH } = workspaceSize()
  const usableW = wsW - ICON_PAD * 2
  const usableH = wsH - ICON_PAD * 2
  const maxCols = Math.max(1, Math.floor(usableW / ICON_CELL_W))
  let rows = Math.max(1, Math.floor(usableH / ICON_CELL_H))
  let cols = Math.ceil(count / rows)
  if (cols > maxCols) {
    cols = maxCols
    rows = Math.max(1, Math.ceil(count / cols))
  }

  const positions = []
  for (let i = 0; i < count; i++) {
    const row = i % rows
    const col = Math.floor(i / rows)
    positions.push({
      x: ICON_PAD + col * ICON_CELL_W,
      y: ICON_PAD + row * ICON_CELL_H,
    })
  }
  return { rows, cols, positions }
}
