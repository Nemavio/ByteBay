import { getContext, setContext } from 'svelte'

const KEY = 'bytebay-desktop'

/** @typedef {{ openCustomWindow: (opts: object) => void }} DesktopApi */

export function setDesktop(api) {
  setContext(KEY, api)
}

/** @returns {DesktopApi} */
export function useDesktop() {
  return getContext(KEY)
}
