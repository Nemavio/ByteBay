import { getContext, setContext } from 'svelte'

const KEY = 'bytebay-window-host'

/** @typedef {{ x: number, y: number, w: number, h: number, key: string }} WindowHost */

/** @param {WindowHost} host */
export function setWindowHost(host) {
  setContext(KEY, host)
}

/** @returns {WindowHost | undefined} */
export function useWindowHost() {
  return getContext(KEY)
}
