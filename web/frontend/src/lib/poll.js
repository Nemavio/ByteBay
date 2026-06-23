/** Intervalle par défaut pour les fenêtres de détail (resync, température, etc.). */
export const POLL_DETAIL_MS = 4000

/** Intervalle pour les listes d'info (SMART, RAID, tableau de bord). */
export const POLL_LIST_MS = 5000

/**
 * Appelle fn immédiatement puis toutes les intervalMs jusqu'au cleanup.
 * @param {() => void | Promise<void>} fn
 * @param {number} intervalMs
 * @returns {() => void}
 */
export function poll(fn, intervalMs) {
  let stopped = false
  let timer

  const schedule = () => {
    timer = setTimeout(async () => {
      if (stopped) return
      try {
        await fn()
      } catch {
        /* erreurs gérées dans fn */
      }
      if (!stopped) schedule()
    }, intervalMs)
  }

  schedule()
  return () => {
    stopped = true
    clearTimeout(timer)
  }
}
