const TEXT_EXT = new Set([
  'txt', 'md', 'log', 'json', 'csv', 'xml', 'html', 'htm', 'css', 'js', 'mjs', 'cjs',
  'ts', 'tsx', 'jsx', 'svelte', 'vue', 'py', 'sh', 'bash', 'yaml', 'yml', 'toml', 'ini',
  'conf', 'cfg', 'env', 'sql', 'rs', 'go', 'java', 'c', 'cpp', 'h', 'hpp', 'php', 'rb',
  'swift', 'kt', 'properties', 'gitignore', 'dockerignore', 'makefile',
])

const IMAGE_EXT = new Set([
  'jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'svg', 'avif', 'heic', 'heif', 'tif', 'tiff', 'ico',
])

const AUDIO_EXT = new Set([
  'mp3', 'ogg', 'oga', 'wav', 'm4a', 'aac', 'flac', 'opus', 'weba',
])

const VIDEO_EXT = new Set([
  'mp4', 'm4v', 'webm', 'mkv', 'mov', 'avi', 'ogv', 'mpeg', 'mpg',
])

const MIME_BY_EXT = {
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  png: 'image/png',
  gif: 'image/gif',
  webp: 'image/webp',
  bmp: 'image/bmp',
  svg: 'image/svg+xml',
  avif: 'image/avif',
  heic: 'image/heic',
  heif: 'image/heif',
  tif: 'image/tiff',
  tiff: 'image/tiff',
  ico: 'image/x-icon',
  mp3: 'audio/mpeg',
  ogg: 'audio/ogg',
  oga: 'audio/ogg',
  wav: 'audio/wav',
  m4a: 'audio/mp4',
  aac: 'audio/aac',
  flac: 'audio/flac',
  opus: 'audio/opus',
  weba: 'audio/webm',
  mp4: 'video/mp4',
  m4v: 'video/mp4',
  webm: 'video/webm',
  mkv: 'video/x-matroska',
  mov: 'video/quicktime',
  avi: 'video/x-msvideo',
  ogv: 'video/ogg',
  mpeg: 'video/mpeg',
  mpg: 'video/mpeg',
}

export function fileExt(name) {
  const base = name.split('/').pop() || name
  const i = base.lastIndexOf('.')
  if (i <= 0) return ''
  return base.slice(i + 1).toLowerCase()
}

export function previewKind(name) {
  const base = (name.split('/').pop() || name).toLowerCase()
  if (base === 'makefile' || base === 'dockerfile' || base === 'readme') return 'text'
  const ext = fileExt(name)
  if (!ext) return null
  if (ext === 'makefile' || TEXT_EXT.has(ext)) return 'text'
  if (IMAGE_EXT.has(ext)) return 'image'
  if (AUDIO_EXT.has(ext)) return 'audio'
  if (VIDEO_EXT.has(ext)) return 'video'
  return null
}

export function mimeFromName(name, headerType = '') {
  if (headerType && headerType !== 'application/octet-stream') return headerType.split(';')[0].trim()
  const ext = fileExt(name)
  return MIME_BY_EXT[ext] || ''
}

export function isHeic(name) {
  const ext = fileExt(name)
  return ext === 'heic' || ext === 'heif'
}

export async function blobForPreview(res, name) {
  const raw = await res.blob()
  const mime = mimeFromName(name, res.headers.get('Content-Type') || raw.type)
  if (!mime || raw.type === mime) return raw
  const buf = await raw.arrayBuffer()
  return new Blob([buf], { type: mime })
}

export async function heicToPreviewUrl(blob) {
  const heic2any = (await import('heic2any')).default
  const converted = await heic2any({ blob, toType: 'image/jpeg', quality: 0.92 })
  const out = Array.isArray(converted) ? converted[0] : converted
  return URL.createObjectURL(out)
}
