// FileRef is a file or folder a chat message names: its path as written, which the app resolves from the thread's
// folder, and the place in it the message points at. Lines count from 1.
export type FileRef = { path: string; line?: number; col?: number; endLine?: number }

const SCHEME = /^([a-z][a-z0-9+.-]*):/i
const NOT_FILE_SCHEMES = new Set(['http', 'https', 'mailto', 'tel', 'ftp', 'ws', 'wss', 'data', 'javascript', 'blob', 'sms', 'ssh', 'git'])
const POSITION_ONLY = /^\d+(?::\d+)?(?:[-–]\d+)?$/
const PATH_POSITION = /:(\d+)(?::(\d+))?(?:[-–](\d+))?$/
const FRAGMENT_POSITION = /^#L(\d+)(?:C(\d+))?(?:[-–]L?(\d+)(?:C(\d+))?)?$/i
const ELLIPSIS_SEGMENT = /^\.{3,}$|…/
const VERSION = /^v?\d+(?:\.\d+)+$/i
const HOST_ENDINGS = new Set(['com', 'org', 'net', 'io', 'dev', 'ai', 'co', 'edu', 'gov', 'info', 'biz', 'xyz', 'uk', 'eu', 'us', 'ru'])
const EXTENSION = /\.[a-z0-9_+-]*[a-z][a-z0-9_+-]*$/i
const WEB_ADDRESS = /^[a-z][a-z0-9+.-]*:\/\//i
const CODE_FORBIDDEN = /[<>`*?|{}]/

// Names of files that have no extension.
const KNOWN_NAMES = new Set(['Makefile', 'GNUmakefile', 'Dockerfile', 'Containerfile', 'Rakefile', 'Gemfile', 'Procfile', 'Vagrantfile', 'Justfile', 'Jenkinsfile', 'Brewfile', 'Podfile', 'Caddyfile', 'Tiltfile', 'Earthfile', 'LICENSE', 'LICENCE', 'README', 'CHANGELOG', 'CODEOWNERS', 'NOTICE', 'AUTHORS', 'COPYING', 'CONTRIBUTING', 'OWNERS', 'WORKSPACE', 'BUILD'])

export const baseOf = (path: string) => path.slice(path.lastIndexOf('/') + 1)

const decoded = (s: string): string | null => {
  try {
    return decodeURIComponent(s)
  } catch {
    return null
  }
}

const number = (s: string | undefined) => (s === undefined ? 0 : Number(s))

// withPlace is the ref of path with the place a suffix or fragment gave; a zero is no place at all.
function withPlace(path: string, line: number, col: number, end: number): FileRef {
  if (line === 0) return { path }
  const ref: FileRef = { path, line }
  if (col > 0) ref.col = col
  if (end > 0 && end !== line) {
    if (end > line) ref.endLine = end
    else return { path, line: end, endLine: line }
  }
  return ref
}

// splitPlace takes the place off the end of a path: a :line, :line:col or :start-end suffix, which wins over a #L42,
// #L42C7 or #L42-L64 fragment; fragment is the text after the path's #, or ''. A fragment that is no place is dropped.
function splitPlace(path: string, fragment: string): FileRef {
  const suffix = PATH_POSITION.exec(path)
  if (suffix) {
    const [text, line, second, end] = suffix
    const hasEnd = end !== undefined
    return withPlace(path.slice(0, -text.length), number(line), hasEnd ? 0 : number(second), hasEnd ? number(end) : 0)
  }
  const frag = FRAGMENT_POSITION.exec(fragment)
  if (frag) return withPlace(path, number(frag[1]), number(frag[2]), number(frag[3]))
  return { path }
}

// sound says a path is worth looking up: it has no empty, ... or … step, and no step that makes it a web address.
function sound(path: string): boolean {
  if (path === '' || path.includes('\0')) return false
  return !path.split('/').some((step) => ELLIPSIS_SEGMENT.test(step))
}

const hostLike = (segment: string) => {
  if (segment === 'localhost' || VERSION.test(segment)) return true
  const dot = segment.lastIndexOf('.')
  return dot > 0 && HOST_ENDINGS.has(segment.slice(dot + 1).toLowerCase())
}

const explicit = (path: string) => path.startsWith('/') || path.startsWith('./') || path.startsWith('../') || path.startsWith('~/')

// hasFileName says the last step of a path is a file's name: it has an extension, or is a name files have with none.
function hasFileName(path: string): boolean {
  const name = baseOf(path)
  return KNOWN_NAMES.has(name) || (EXTENSION.test(name) && !VERSION.test(name) && !name.endsWith('.'))
}

// linkRef is the file or folder a markdown link's address names, null for a web address or any other thing. An address
// is a path (absolute, ./, ../, ~/ or relative) or a file: URL, with a place after it, and a relative path must look
// like a file or lie in a folder.
export function linkRef(href: string): FileRef | null {
  if (href === '' || href.startsWith('#') || href.startsWith('//')) return null
  let raw: string
  let fragment: string
  if (/^file:/i.test(href)) {
    let url: URL
    try {
      url = new URL(href)
    } catch {
      return null
    }
    if (url.host !== '' && url.host !== 'localhost') return null
    const path = decoded(url.pathname)
    if (path === null) return null
    raw = path
    fragment = decoded(url.hash) ?? ''
  } else {
    const scheme = SCHEME.exec(href)
    if (scheme && !(POSITION_ONLY.test(href.slice(scheme[0].length)) && !NOT_FILE_SCHEMES.has(scheme[1].toLowerCase()))) return null
    const hash = href.indexOf('#')
    const head = hash < 0 ? href : href.slice(0, hash)
    const path = decoded(head.replace(/\?.*$/, ''))
    if (path === null) return null
    raw = path
    fragment = hash < 0 ? '' : (decoded(href.slice(hash)) ?? '')
  }
  const ref = splitPlace(raw, fragment)
  const path = trimmed(ref.path)
  if (path === null || !sound(path) || !fits(path)) return null
  return { ...ref, path }
}

// trimmed is the path without its slash at the end, null for a path that is only slashes or dots, or only ~.
function trimmed(path: string): string | null {
  const text = path.length > 1 ? path.replace(/\/+$/, '') : path
  if (text === '' || text === '/' || text === '.' || text === '..' || text === '~') return null
  return text
}

// fits says a link's path is a place and not a word: it is explicit, or it has a folder or a file's name; and a first
// step that is a host does not make a relative path.
function fits(path: string): boolean {
  if (explicit(path)) return true
  const steps = path.split('/')
  return !hostLike(steps[0]) && (steps.length > 1 || hasFileName(path))
}

// codeRef is the file an inline code span names, null for any other code: a path with a / or a :line, no space unless
// it is an absolute path, none of < > ` * ? | { }, no slash at the end, no host or version for its first step, and a file's
// name for its last.
export function codeRef(code: string): FileRef | null {
  if (code === '' || CODE_FORBIDDEN.test(code) || WEB_ADDRESS.test(code)) return null
  if (/\s/.test(code) && !(code.startsWith('/') && !/^\s|\s$/.test(code) && !/[\r\n\t]/.test(code))) return null
  const hash = code.indexOf('#')
  if (hash >= 0 && !FRAGMENT_POSITION.test(code.slice(hash))) return null
  const ref = hash < 0 ? splitPlace(code, '') : splitPlace(code.slice(0, hash), code.slice(hash))
  const placed = ref.line !== undefined
  const path = ref.path
  if (path === '' || path.endsWith('/') || !sound(path)) return null
  if (!path.includes('/') && !placed) return null
  if (!explicit(path)) {
    const steps = path.split('/')
    if (hostLike(steps[0])) return null
  }
  if (!hasFileName(path)) return null
  return ref
}

// refText is a ref written as the app copies it: its path and a :line, :line:col or :start-end suffix.
export function refText(path: string, ref: Pick<FileRef, 'line' | 'col' | 'endLine'>): string {
  if (ref.line === undefined) return path
  if (ref.endLine !== undefined) return `${path}:${ref.line}-${ref.endLine}`
  return ref.col === undefined ? `${path}:${ref.line}` : `${path}:${ref.line}:${ref.col}`
}

// placeLabel is the place of a ref as a chip shows it after a name: :42 or :42–64, '' for none.
export function placeLabel(ref: Pick<FileRef, 'line' | 'endLine'>): string {
  if (ref.line === undefined) return ''
  return ref.endLine === undefined ? `:${ref.line}` : `:${ref.line}–${ref.endLine}`
}

// chipNames is the name each of the message's paths shows: its last step, with as many folders before it as it takes to
// tell it from the other paths that end in the same name. The same path twice gets one name.
export function chipNames(paths: string[]): Map<string, string> {
  const byName = new Map<string, Set<string>>()
  for (const path of paths) byName.set(baseOf(path), (byName.get(baseOf(path)) ?? new Set()).add(path))
  const tail = (path: string, n: number) => path.split('/').filter(Boolean).slice(-n).join('/')
  const names = new Map<string, string>()
  for (const group of byName.values()) {
    for (const path of group) {
      const steps = path.split('/').filter(Boolean).length
      let depth = 1
      while (group.size > 1 && depth < steps && (depth === 1 || [...group].some((other) => other !== path && tail(other, depth) === tail(path, depth)))) depth++
      names.set(path, tail(path, depth))
    }
  }
  return names
}
