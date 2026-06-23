const TOKEN_KEY = 'bytebay_token'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(t) {
  localStorage.setItem(TOKEN_KEY, t)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

function formatApiError(err, fallback) {
  const raw = err?.error ?? err?.detail ?? err?.message
  if (typeof raw === 'string') return raw
  if (Array.isArray(raw)) {
    return raw.map((e) => e?.msg || JSON.stringify(e)).join(' · ')
  }
  if (raw && typeof raw === 'object') return JSON.stringify(raw)
  return fallback || 'Erreur'
}

async function request(path, opts = {}) {
  const headers = { ...opts.headers }
  if (!(opts.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
  }
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`
  const res = await fetch(path, { ...opts, headers })
  if (res.status === 401) {
    clearToken()
    throw new Error('unauthorized')
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(formatApiError(err, res.statusText))
  }
  if (res.status === 204) return null
  return res.json()
}

export const api = {
  login: (username, password) =>
    request('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  me: () => request('/api/v1/auth/me'),
  dashboard: () => request('/api/v1/dashboard'),
  disks: () => request('/api/v1/disks'),
  smart: (dev) => request(`/api/v1/disks/${encodeURIComponent(dev)}/smart`),
  smartAll: () => request('/api/v1/smart'),
  smartAlerts: () => request('/api/v1/smart/alerts'),
  raid: () => request('/api/v1/raid'),
  raidDetail: (name) => request(`/api/v1/raid/${encodeURIComponent(name)}`),
  raidCreate: (body) => request('/api/v1/raid', { method: 'POST', body: JSON.stringify(body) }),
  raidJob: (id) => request(`/api/v1/raid/jobs/${encodeURIComponent(id)}`),
  housekeeping: () => request('/api/v1/housekeeping'),
  recoverRaid: (uuid, force = true) =>
    request('/api/v1/housekeeping/recover-raid', {
      method: 'POST',
      body: JSON.stringify({ uuid, force }),
    }),
  raidAdd: (name, device) =>
    request(`/api/v1/raid/${encodeURIComponent(name)}/add`, {
      method: 'POST',
      body: JSON.stringify({ device }),
    }),
  raidSync: (name, action) =>
    request(`/api/v1/raid/${encodeURIComponent(name)}/sync`, {
      method: 'POST',
      body: JSON.stringify({ action }),
    }),
  raidStop: (name, password) =>
    request(`/api/v1/raid/${encodeURIComponent(name)}`, {
      method: 'DELETE',
      body: JSON.stringify({ password }),
    }),
  mounts: () => request('/api/v1/mounts'),
  mountCreate: (body) => request('/api/v1/mounts', { method: 'POST', body: JSON.stringify(body) }),
  mountJob: (id) => request(`/api/v1/mounts/jobs/${encodeURIComponent(id)}`),
  mountDelete: (name) => request(`/api/v1/mounts/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  volumes: () => request('/api/v1/volumes'),
  network: () => request('/api/v1/network'),
  networkPut: (body) => request('/api/v1/network', { method: 'PUT', body: JSON.stringify(body) }),
  networkApply: () => request('/api/v1/network/apply', { method: 'POST', body: '{}' }),
  shares: () => request('/api/v1/shares'),
  sharesPut: (kind, body) =>
    request(`/api/v1/shares/${kind}`, { method: 'PUT', body: JSON.stringify(body) }),
  sharesApply: () => request('/api/v1/shares/apply', { method: 'POST', body: '{}' }),
  users: () => request('/api/v1/users'),
  userCreate: (body) => request('/api/v1/users', { method: 'POST', body: JSON.stringify(body) }),
  userUpdate: (id, body) =>
    request(`/api/v1/users/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  userDelete: (id) => request(`/api/v1/users/${id}`, { method: 'DELETE' }),
  acl: () => request('/api/v1/acl'),
  aclCreate: (body) => request('/api/v1/acl', { method: 'POST', body: JSON.stringify(body) }),
  aclDelete: (id) => request(`/api/v1/acl/${id}`, { method: 'DELETE' }),
  files: (path) => request(`/api/v1/files?path=${encodeURIComponent(path)}`),
  filesMkdir: (path) => request('/api/v1/files/mkdir', { method: 'POST', body: JSON.stringify({ path }) }),
  filesUpload: async (dirPath, file) => {
    const fd = new FormData()
    fd.append('file', file)
    const token = getToken()
    const res = await fetch(`/api/v1/files/upload?path=${encodeURIComponent(dirPath)}`, {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: fd,
    })
    if (!res.ok) throw new Error((await res.json().catch(() => ({}))).detail || res.statusText)
    return res.json()
  },
  fileUrl: (path) => `/api/v1/files/download?path=${encodeURIComponent(path)}`,
  logs: (since = '', sources = '') => {
    const q = new URLSearchParams()
    if (since) q.set('since', since)
    if (sources) q.set('sources', sources)
    const qs = q.toString()
    return request(`/api/v1/logs${qs ? `?${qs}` : ''}`)
  },
  logSources: () => request('/api/v1/logs/sources'),
}
