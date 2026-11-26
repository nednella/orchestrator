// Where the Files view keeps whether it shows a file rendered or as its source.
export const FILES_MODE_KEY = 'agentos.filesMode'
export type FilesMode = 'rendered' | 'source'

// The picker's entry for the plan Claude last asked to have approved; no file path can be this, as each ends in .md.
export const LATEST_PLAN = ':latest-plan'

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

// fileLabel is how the picker names a file: its folders, '' for none, and its name.
export function fileLabel(path: string): { folders: string; name: string } {
  if (path === LATEST_PLAN) return { folders: '', name: 'Latest plan' }
  const parts = path.split('/').filter(Boolean)
  const name = parts.pop() ?? ''
  return { folders: parts.join(' / '), name }
}

// pickerOrder is the order the picker lists files in: AGENTS.md, README.md, then the files under docs/, then the rest,
// each alphabetical.
export function pickerOrder(files: string[]): string[] {
  const rank = (f: string) => (f === 'AGENTS.md' ? 0 : f === 'README.md' ? 1 : f.startsWith('docs/') ? 2 : 3)
  return [...files].sort((a, b) => rank(a) - rank(b) || a.localeCompare(b))
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
