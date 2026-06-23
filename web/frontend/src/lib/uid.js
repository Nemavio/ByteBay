/** ID unique — randomUUID exige HTTPS (NAS en http://IP) */
export function uid() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    try {
      return crypto.randomUUID()
    } catch {
      /* secure context requis */
    }
  }
  return `w-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`
}
