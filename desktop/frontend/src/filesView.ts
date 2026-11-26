// Where the Files view keeps whether it shows a markdown file rendered, how it wraps lines, whether its tree shows and
// how wide.
export const FILES_RENDERED_KEY = 'orchestrator.filesRendered'
export const FILES_WRAP_KEY = 'orchestrator.filesWrap'
export const FILES_TREE_KEY = 'orchestrator.filesTree'
export const FILES_TREE_WIDTH_KEY = 'orchestrator.filesTreeWidth'

const MARKDOWN = /\.(md|markdown)$/i
const SCHEME = /^[a-z][a-z0-9+.-]*:/i

const decoded = (s: string): string | null => {
  try {
    return decodeURIComponent(s)
  } catch {
    return null
  }
}

// planPath is the markdown file a link points at: a relative path, a file:// URL or an absolute path that ends in .md
// or .markdown, without its #fragment or ?query. Web and mail links, any other scheme and other files give null.
export function planPath(href: string): string | null {
  let path: string | null
  if (/^file:/i.test(href)) {
    let url: URL
    try {
      url = new URL(href)
    } catch {
      return null
    }
    if (url.host !== '' && url.host !== 'localhost') return null
    path = decoded(url.pathname)
  } else {
    if (SCHEME.test(href) || href.startsWith('//')) return null
    path = decoded(href.replace(/[?#].*$/, ''))
  }
  return path !== null && MARKDOWN.test(path) ? path : null
}

// linkTarget is the path a link in the file at from points at: an absolute path as it is, a relative one from the
// file's folder, its . and .. steps taken. A .. past the top stays, for the app to refuse.
export function linkTarget(from: string, path: string): string {
  if (path.startsWith('/')) return path
  const parts = from.split('/').slice(0, -1)
  for (const step of path.split('/')) {
    if (step === '' || step === '.') continue
    if (step === '..' && parts.length > 0 && parts.at(-1) !== '..') parts.pop()
    else parts.push(step)
  }
  return parts.join('/')
}

// The file each Files view was asked to show, by its thread's id or by filesKey for Home, while the app runs.
const picked = new Map<string, string>()
const listeners = new Set<() => void>()

export const filesKey = (thread: string | undefined, project: string) => thread ?? `project:${project}`

export function pickFile(key: string, path: string) {
  picked.set(key, path)
  listeners.forEach((listener) => listener())
}

export const pickedFile = (key: string): string | null => picked.get(key) ?? null

export function onPicked(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

// imageTarget is the path of the image a markdown file at from names by src, without its #fragment or ?query: a relative
// path, from the file's folder. A web address, any other scheme and an absolute path give null.
export function imageTarget(from: string, src: string): string | null {
  if (SCHEME.test(src) || src.startsWith('/')) return null
  const path = decoded(src.replace(/[?#].*$/, ''))
  return path === null || path === '' ? null : linkTarget(from, path)
}
