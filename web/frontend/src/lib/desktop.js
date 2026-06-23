import { getContext, setContext } from 'svelte'

const KEY = 'bytebay-desktop'

/** @typedef {{ openCustomWindow: (opts: object) => void, openAppById: (id: string) => void, closeWindow: (key: string) => void }} DesktopApi */

export function setDesktop(api) {
  setContext(KEY, api)
}

/** @returns {DesktopApi} */
export function useDesktop() {
  return getContext(KEY)
}
