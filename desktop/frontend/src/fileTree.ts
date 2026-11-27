import type { FileTree, FileTreeDirectoryHandle } from '@pierre/trees'
import type { FileEntry } from './types'

const MARKDOWN = /\.(md|mdx|markdown|mdown|mkd)$/i

export const isMarkdownPath = (path: string) => MARKDOWN.test(path)

// treePath is how the tree names an entry: a folder ends in a slash.
export const treePath = (e: FileEntry) => (e.dir ? `${e.path}/` : e.path)

// ancestorsOf is the tree paths of the folders a path is in, outermost first.
export function ancestorsOf(path: string): string[] {
  const parts = path.replace(/\/$/, '').split('/')
  return parts.slice(0, -1).map((_, i) => `${parts.slice(0, i + 1).join('/')}/`)
}

// crumbsOf is the names that lead to a path, and the path of the folder each one ends at, '' for the file itself.
export function crumbsOf(path: string): { name: string; folder: string }[] {
  const parts = path.split('/').filter(Boolean)
  return parts.map((name, i) => ({ name, folder: i < parts.length - 1 ? parts.slice(0, i + 1).join('/') : '' }))
}

// formatBytes is a size in bytes as the file view says it.
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(n < 10 * 1024 ? 1 : 0)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

// withFolders is the list with the folders its paths lead through, which git does not list.
export function withFolders(entries: FileEntry[]): FileEntry[] {
  const seen = new Set(entries.filter((e) => e.dir).map((e) => e.path))
  const added: FileEntry[] = []
  for (const e of entries) {
    for (const folder of ancestorsOf(e.path)) {
      const path = folder.slice(0, -1)
      if (seen.has(path)) continue
      seen.add(path)
      added.push({ path, dir: true, ignored: false })
    }
  }
  return added.length === 0 ? entries : [...entries, ...added]
}

// childrenOf is what the list holds directly in a folder, '' for the top, folders first, each by name.
export function childrenOf(entries: FileEntry[], folder: string): FileEntry[] {
  const prefix = folder === '' ? '' : `${folder}/`
  return entries
    .filter((e) => e.path.startsWith(prefix) && !e.path.slice(prefix.length).includes('/'))
    .sort((a, b) => Number(b.dir) - Number(a.dir) || a.path.localeCompare(b.path))
}

export const baseName = (path: string) => path.slice(path.lastIndexOf('/') + 1)

// parentOf is the folder a path is in, '' for the top.
export const parentOf = (path: string) => path.slice(0, Math.max(0, path.lastIndexOf('/')))

// folderHandle is the tree's handle on the folder at a tree path, null for a path the tree does not list or a file.
export function folderHandle(model: FileTree, path: string): FileTreeDirectoryHandle | null {
  const item = model.getItem(path)
  return item?.isDirectory() ? (item as FileTreeDirectoryHandle) : null
}

// withoutFrontMatter is the text of a markdown file without the block of settings that may open it.
export const withoutFrontMatter = (text: string) => text.replace(/^---[ \t]*\r?\n[\s\S]*?\r?\n---[ \t]*(?:\r?\n|$)/, '')

// relativeTo is a path as the tree names it: relative to folder when it is inside it, else as it was given.
export const relativeTo = (folder: string, path: string) => (folder !== '' && path.startsWith(`${folder}/`) ? path.slice(folder.length + 1) : path)

// onlyChildFolders is each folder of the list that holds nothing but one folder, with that folder's path.
export function onlyChildFolders(entries: FileEntry[]): Map<string, string> {
  const children = new Map<string, FileEntry[]>()
  for (const e of entries) {
    const parent = parentOf(e.path)
    const held = children.get(parent)
    if (held) held.push(e)
    else children.set(parent, [e])
  }
  const only = new Map<string, string>()
  for (const [folder, held] of children) if (folder !== '' && held.length === 1 && held[0].dir) only.set(folder, held[0].path)
  return only
}

// folderRow is the folder that names the tree's row of a folder: a folder that holds only a folder shares one row with
// it, named by the innermost of them. onlyChild is what onlyChildFolders gives.
export function folderRow(onlyChild: Map<string, string>, folder: string): string {
  let row = folder
  for (let inner = onlyChild.get(row); inner !== undefined; inner = onlyChild.get(row)) row = inner
  return row
}
