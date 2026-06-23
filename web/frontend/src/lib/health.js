/** @typedef {{ web: boolean, agent: boolean, engine: boolean }} Health */

/** @param {Record<string, unknown>} raw */
export function normalizeHealth(raw) {
  return {
    web: raw.web === true || raw.web === 'ok',
    agent: !!raw.agent,
    engine: !!raw.engine,
  }
}

/**
 * Interroge /api/v1/health avec une nouvelle tentative après échec transitoire.
 * @param {() => Promise<Record<string, unknown>>} fetchHealth
 * @returns {Promise<Health | null>}
 */
export async function fetchHealthWithRetry(fetchHealth) {
  for (let attempt = 0; attempt < 2; attempt++) {
    if (attempt > 0) {
      await new Promise((r) => setTimeout(r, 400))
    }
    try {
      return normalizeHealth(await fetchHealth())
    } catch {
      /* retry */
    }
  }
  return null
}
