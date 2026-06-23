/** Ferme la modale overlay la plus récente, ou indique qu'aucune n'est ouverte. */
export function dismissTopOverlay() {
  const overlays = document.querySelectorAll('.overlay')
  if (!overlays.length) return false
  const top = overlays[overlays.length - 1]
  const ghost = top.querySelector('button.ghost')
  if (ghost) {
    ghost.click()
    return true
  }
  top.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  return true
}
