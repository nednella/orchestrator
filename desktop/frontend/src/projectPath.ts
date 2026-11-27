import type { FolderEntry, FolderListing } from './types'

const hasTrailingSlash = (path: string) => path.endsWith('/')

// A path the folder browser lists: absolute, in the home folder, or in the current project.
export const isBrowsePath = (value: string) => value.startsWith('/') || value.startsWith('~/') || value.startsWith('./') || value.startsWith('../')

export const needsProject = (value: string) => value.startsWith('./') || value.startsWith('../')

// The folder a path lists: the path itself when it ends in a slash, else everything up to its last slash.
export function directoryOf(path: string): string {
  if (hasTrailingSlash(path)) return path
  const slash = path.lastIndexOf('/')
  return slash < 0 ? path : path.slice(0, slash + 1)
}

// What is typed after the last slash, which filters the folder's list.
export const tailOf = (path: string) => path.slice(path.lastIndexOf('/') + 1)

// The folder above a folder path, or null when there is none to go to.
export function parentOf(directory: string): string | null {
  const trimmed = directory.replace(/\/+$/, '')
  if (directory.startsWith('/')) {
    const segments = trimmed.split('/').filter(Boolean)
    if (segments.length === 0) return null
    return segments.length === 1 ? '/' : `/${segments.slice(0, -1).join('/')}/`
  }
  const slash = trimmed.lastIndexOf('/')
  return slash < 0 ? null : trimmed.slice(0, slash + 1)
}

export const enter = (current: string, name: string) => `${directoryOf(current)}${name}/`

type Scored = { entry: FolderEntry; score: number }

function score(name: string, query: string): number | null {
  if (name === query) return 0
  if (name.startsWith(query)) return 1000 + name.length - query.length
  const at = name.indexOf(query)
  if (at >= 0) return (/[ \-_.]/.test(name[at - 1] ?? '') ? 2000 : 3000) + at
  let from = 0
  for (const ch of query) {
    from = name.indexOf(ch, from) + 1
    if (from === 0) return null
  }
  return 4000 + name.length
}

// The folders that match what is typed, best first, and the one the typed name is exactly. A name that starts with a dot
// shows only when the typed name does. Finder stores names decomposed, so both forms are compared.
export function matchFolders(entries: FolderEntry[], query: string): { visible: FolderEntry[]; exact: FolderEntry | null } {
  const showHidden = query.startsWith('.')
  const candidates = entries.filter((e) => showHidden || !e.name.startsWith('.'))
  const lower = query.toLowerCase()
  const composed = query.normalize('NFC').toLowerCase()
  const visible =
    lower === ''
      ? candidates
      : candidates
          .map((entry): Scored | null => {
            const name = entry.name.toLowerCase()
            const a = score(name, lower)
            const b = score(entry.name.normalize('NFC').toLowerCase(), composed)
            const best = a === null ? b : b === null ? a : Math.min(a, b)
            return best === null ? null : { entry, score: best }
          })
          .filter((m): m is Scored => m !== null)
          .sort((x, y) => x.score - y.score)
          .map((m) => m.entry)
  if (query === '') return { visible, exact: null }
  const same = visible.filter((e) => e.name.normalize('NFC') === query.normalize('NFC'))
  return { visible, exact: visible.find((e) => e.name === query) ?? (same.length === 1 ? same[0] : null) }
}

// Tab as in a shell: one match goes into its folder, several extend what is typed to the start they share.
export function completeFolder(visible: FolderEntry[], tail: string): { name: string; whole: boolean } | null {
  const lower = tail.toLowerCase()
  const composed = tail.normalize('NFC').toLowerCase()
  const starts = visible.filter((e) => e.name.toLowerCase().startsWith(lower) || e.name.normalize('NFC').toLowerCase().startsWith(composed))
  if (starts.length === 0) return null
  if (starts.length === 1) return { name: starts[0].name, whole: true }
  const shared = sharedStart(starts.map((e) => e.name), lower) ?? sharedStart(starts.map((e) => e.name.normalize('NFC')), composed)
  return shared === null ? null : { name: shared, whole: false }
}

function sharedStart(names: string[], lowerTail: string): string | null {
  let prefix = Array.from(names[0])
  for (const name of names.slice(1)) {
    const chars = Array.from(name)
    let n = 0
    while (n < prefix.length && prefix[n].toLowerCase() === chars[n]?.toLowerCase()) n++
    prefix = prefix.slice(0, n)
  }
  const shared = prefix.join('')
  const longer = shared.toLowerCase().length > lowerTail.length && names.every((n) => n.toLowerCase().startsWith(shared.toLowerCase()))
  return longer ? shared : null
}

const SPECIAL_FOLDS: Record<string, string> = {
  Ŀ: 'L', ŀ: 'l', ŉ: 'n', Đ: 'D', đ: 'd', Ð: 'D', ð: 'd', ß: 'ss', ẞ: 'SS', Ł: 'L', ł: 'l', Ø: 'O', ø: 'o', Æ: 'AE', æ: 'ae', Œ: 'OE', œ: 'oe',
  Þ: 'TH', þ: 'th', Ħ: 'H', ħ: 'h', ı: 'i', ĸ: 'q', Ŋ: 'N', ŋ: 'n', Ŧ: 'T', ŧ: 't',
}
const SPECIAL = new RegExp(`[${Object.keys(SPECIAL_FOLDS).join('')}]`, 'g')

// The name of the folder a new project is made in; desktop/projects.FolderName does the same, and both are tested against
// desktop/projects/testdata/folder-names.json.
export function folderName(name: string): string {
  const slug = name
    .replace(SPECIAL, (c) => SPECIAL_FOLDS[c])
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+/, '')
    .slice(0, 64)
    .replace(/-+$/, '')
  return slug === '' ? 'project' : slug
}

export const projectPath = (root: string, name: string) => `${root.replace(/\/+$/, '')}/${folderName(name)}`

// The GitHub repository a project in path is published as.
export const repositoryOf = (owner: string, path: string) => `${owner}/${path.slice(path.lastIndexOf('/') + 1)}`

export type BrowsePlan = {
  // directory is the folder listed and tail what is typed after it, which filters the list.
  directory: string
  tail: string
  parent: string | null
  visible: FolderEntry[]
  // path is what Add adds: the folder itself when the path ends in a slash, else the folder the typed name is, else the
  // typed text, which becomes a new folder.
  path: string
  // creates says Add makes the folder, because nothing is there yet.
  creates: boolean
}

export function planBrowse(query: string, listing: FolderListing | null): BrowsePlan {
  const directory = directoryOf(query)
  const trailing = hasTrailingSlash(query)
  const tail = trailing ? '' : tailOf(query)
  const { visible, exact } = matchFolders(listing?.entries ?? [], tail)
  const typed = query.trim()
  return {
    directory,
    tail,
    parent: parentOf(directory),
    visible,
    path: trailing ? (listing?.dir ?? typed) : (exact?.path ?? typed),
    creates: typed !== '' && (trailing ? !listing?.exists : exact === null),
  }
}
