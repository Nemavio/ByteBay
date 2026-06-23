import { getToken, api } from './api.js'

const BASE = '/api/v1/webdav'
const DAV_NS = 'DAV:'

function authHeaders(extra = {}) {
  const headers = { ...extra }
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`
  return headers
}

/** @param {string} fsPath e.g. /volumes/foo */
export function davUrl(fsPath) {
  if (!fsPath || fsPath === '/') return BASE
  return BASE + encodeURI(fsPath)
}

function hrefToFsPath(href) {
  if (!href) return null
  try {
    const path = new URL(href, window.location.origin).pathname
    if (!path.startsWith(BASE)) return null
    const rest = path.slice(BASE.length) || '/volumes'
    return decodeURIComponent(rest)
  } catch {
    return null
  }
}

function childEl(parent, localName) {
  for (const node of parent.children) {
    if (node.localName === localName) return node
  }
  return null
}

function parsePropfind(xml, basePath) {
  const doc = new DOMParser().parseFromString(xml, 'application/xml')
  const responses = doc.getElementsByTagNameNS(DAV_NS, 'response')
  /** @type {{ name: string, path: string, is_dir: boolean, size: number, mod_time?: string }[]} */
  const entries = []
  const normBase = basePath.replace(/\/$/, '') || basePath

  for (const resp of responses) {
    const href = childEl(resp, 'href')?.textContent?.trim()
    const fsPath = hrefToFsPath(href)
    if (!fsPath || fsPath === normBase) continue

    const propstat = childEl(resp, 'propstat')
    const prop = propstat ? childEl(propstat, 'prop') : null
    if (!prop) continue

    const display = childEl(prop, 'displayname')?.textContent
    const size = parseInt(childEl(prop, 'getcontentlength')?.textContent || '0', 10) || 0
    const isDir = !!childEl(prop, 'resourcetype')?.getElementsByTagNameNS(DAV_NS, 'collection').length
    const name = display || fsPath.split('/').pop() || ''

    entries.push({
      name,
      path: fsPath,
      is_dir: isDir,
      size,
      mod_time: childEl(prop, 'getlastmodified')?.textContent || undefined,
    })
  }
  return entries
}

/**
 * @param {string} fsPath
 * @returns {Promise<{ name: string, path: string, is_dir: boolean, size: number, mod_time?: string }[]>}
 */
export async function listDirectory(fsPath) {
  try {
    const res = await fetch(davUrl(fsPath), {
      method: 'PROPFIND',
      headers: authHeaders({
        Depth: '1',
        'Content-Type': 'application/xml; charset=utf-8',
      }),
      body:
        '<?xml version="1.0" encoding="utf-8"?>' +
        '<d:propfind xmlns:d="DAV:"><d:prop><d:displayname/>' +
        '<d:getcontentlength/><d:resourcetype/><d:getlastmodified/></d:prop></d:propfind>',
    })
    if (!res.ok) throw new Error(await res.text().catch(() => res.statusText))
    return parsePropfind(await res.text(), fsPath)
  } catch {
    const list = await api.files(fsPath)
    return (Array.isArray(list) ? list : []).filter((e) => e.name !== '..')
  }
}

/**
 * @param {string} fsPath
 * @param {Blob|File} body
 * @param {(loaded: number, total: number) => void} [onProgress]
 */
export function putFile(fsPath, body, onProgress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', davUrl(fsPath))
    const token = getToken()
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
    if (body.type) xhr.setRequestHeader('Content-Type', body.type)
    xhr.upload.onprogress = (e) => {
      if (onProgress && e.lengthComputable) onProgress(e.loaded, e.total)
    }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve()
      else reject(new Error(xhr.responseText || xhr.statusText || 'Upload failed'))
    }
    xhr.onerror = () => reject(new Error('Erreur réseau'))
    xhr.send(body)
  })
}

export async function mkcol(fsPath) {
  try {
    const res = await fetch(davUrl(fsPath), {
      method: 'MKCOL',
      headers: authHeaders(),
    })
    if (res.status === 201 || res.status === 405) return
    if (!res.ok) throw new Error(await res.text().catch(() => res.statusText))
  } catch {
    await api.filesMkdir(fsPath)
  }
}

/** @param {string} fsPath */
export async function deletePath(fsPath) {
  const res = await fetch(davUrl(fsPath), {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (!res.ok && res.status !== 204) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(text || 'Suppression impossible')
  }
}

/** @param {string} from @param {string} to */
export async function movePath(from, to) {
  const destUrl = new URL(davUrl(to), window.location.origin).href
  const res = await fetch(davUrl(from), {
    method: 'MOVE',
    headers: authHeaders({ Destination: destUrl }),
  })
  if (!res.ok && res.status !== 201 && res.status !== 204) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(text || 'Déplacement impossible')
  }
}

/** @param {string} fsPath @param {string} newName */
export async function renamePath(fsPath, newName) {
  const parent = fsPath.replace(/\/[^/]+$/, '') || '/volumes'
  const to = `${parent}/${newName}`
  await movePath(fsPath, to)
  return to
}

/**
 * Upload plusieurs fichiers ou un dossier (webkitRelativePath).
 * @param {string} dirPath
 * @param {File[]} files
 * @param {(info: { index: number, total: number, name: string, loaded: number, fileTotal: number, overallPercent: number }) => void} [onProgress]
 */
export async function uploadFiles(dirPath, files, onProgress) {
  const base = dirPath.replace(/\/$/, '')
  const totalBytes = files.reduce((s, f) => s + f.size, 0) || files.length
  let doneBytes = 0

  for (let i = 0; i < files.length; i++) {
    const file = files[i]
    const rel = file.webkitRelativePath || file.name
    const dest = `${base}/${rel}`
    await putFile(dest, file, (loaded, fileTotal) => {
      const overall = totalBytes > 0 ? ((doneBytes + loaded) / totalBytes) * 100 : ((i + loaded / (fileTotal || 1)) / files.length) * 100
      onProgress?.({
        index: i + 1,
        total: files.length,
        name: rel,
        loaded,
        fileTotal,
        overallPercent: Math.min(100, overall),
      })
    })
    doneBytes += file.size
    onProgress?.({
      index: i + 1,
      total: files.length,
      name: rel,
      loaded: file.size,
      fileTotal: file.size,
      overallPercent: totalBytes > 0 ? (doneBytes / totalBytes) * 100 : ((i + 1) / files.length) * 100,
    })
  }
}

export { davUrl as fileUrl }
