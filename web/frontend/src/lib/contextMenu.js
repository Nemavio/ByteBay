import { getContext, setContext } from 'svelte'

const KEY = 'bytebay-context-menu'

/** @typedef {{ label: string, icon?: string, action?: () => void, danger?: boolean, disabled?: boolean, separator?: boolean }} ContextMenuItem */

/** @param {{ show: (x: number, y: number, items: ContextMenuItem[]) => void, close: () => void }} api */
export function setContextMenu(api) {
  setContext(KEY, api)
}

/** @returns {{ show: (x: number, y: number, items: ContextMenuItem[]) => void, close: () => void }} */
export function useContextMenu() {
  return getContext(KEY)
}

/**
 * @param {MouseEvent} e
 * @param {ContextMenuItem[]} items
 * @param {{ show: (x: number, y: number, items: ContextMenuItem[]) => void }} menu
 */
export function openContextMenu(e, items, menu) {
  e.preventDefault()
  e.stopPropagation()
  menu.show(e.clientX, e.clientY, items)
}
