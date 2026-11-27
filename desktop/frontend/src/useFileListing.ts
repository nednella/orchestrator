import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, errorMessage } from './api'
import type { FileEntry } from './types'

// The folders the Files view lists at once.
const PARALLEL = 4

// What each folder git ignores holds, once the owner has opened it, by the folder's path.
type Folders = Record<string, FileEntry[]>

type Listing = { key: string; entries: FileEntry[]; truncated: boolean; folders: Folders }

type Loaded = { listing: Listing } | { key: string; error: string }

export type FileListingState =
  | { status: 'loading' }
  | { status: 'error'; error: string }
  | { status: 'ready'; entries: FileEntry[]; truncated: boolean; loading: ReadonlySet<string>; refreshing: boolean }

export type FileListingActions = {
  // loadFolder lists a folder git ignores, which the tree lists only once it is opened.
  loadFolder(path: string): void
  refresh(): void
}

// useFileListing lists the files of the thread's folder, or of the project's with no thread, again when refresh is
// called or the thread's state moves, as at a turn's end, when its agent may have written files.
export function useFileListing(thread: string | undefined, project: string, state: string | undefined): [FileListingState, FileListingActions] {
  const key = thread ?? `project:${project}`
  const [loaded, setLoaded] = useState<Loaded | null>(null)
  const [loading, setLoading] = useState<ReadonlySet<string>>(new Set())
  const [refreshing, setRefreshing] = useState(false)
  const [round, setRound] = useState(0)
  const opened = useRef<{ key: string; folders: Folders; reading: Set<string> }>({ key, folders: {}, reading: new Set() })
  if (opened.current.key !== key) opened.current = { key, folders: {}, reading: new Set() }

  const readFolder = useCallback((path: string) => (thread ? api.chatFileDir(thread, path) : api.projectFileDir(project, path)), [thread, project])

  useEffect(() => {
    let stale = false
    const mine = opened.current
    setRefreshing(true)
    const tree = thread ? api.chatFileTree(thread) : api.projectFileTree(project)
    tree
      .then(async (top) => {
        // A folder the owner opened is listed again, and one that cannot be is dropped.
        const queue = Object.keys(mine.folders)
        const folders: Folders = {}
        await Promise.all(
          Array.from({ length: Math.min(PARALLEL, queue.length) }, async () => {
            for (let path = queue.shift(); path !== undefined; path = queue.shift()) {
              try {
                folders[path] = (await readFolder(path)).entries
              } catch {
                // The folder shows closed and empty.
              }
            }
          }),
        )
        if (stale) return
        mine.folders = folders
        setLoaded({ listing: { key, entries: top.entries, truncated: top.truncated, folders } })
      })
      .catch((err) => !stale && setLoaded({ key, error: errorMessage(err) }))
      .finally(() => !stale && setRefreshing(false))
    return () => {
      stale = true
    }
  }, [thread, project, key, state, round, readFolder])

  const loadFolder = useCallback(
    (path: string) => {
      const mine = opened.current
      if (mine.folders[path] !== undefined || mine.reading.has(path)) return
      mine.reading.add(path)
      setLoading((l) => new Set(l).add(path))
      readFolder(path)
        .then((dir) => dir.entries)
        .catch(() => [] as FileEntry[])
        .then((entries) => {
          mine.reading.delete(path)
          mine.folders = { ...mine.folders, [path]: entries }
          setLoaded((l) => (l && 'listing' in l && l.listing.key === mine.key ? { listing: { ...l.listing, folders: mine.folders } } : l))
          setLoading((l) => {
            const next = new Set(l)
            next.delete(path)
            return next
          })
        })
    },
    [readFolder],
  )

  const refresh = useCallback(() => setRound((n) => n + 1), [])
  const actions = useMemo(() => ({ loadFolder, refresh }), [loadFolder, refresh])

  const listing = loaded && 'listing' in loaded && loaded.listing.key === key ? loaded.listing : null
  const entries = useMemo(() => (listing ? [...listing.entries, ...Object.values(listing.folders).flat()] : []), [listing])
  if (loaded && 'error' in loaded && loaded.key === key) return [{ status: 'error', error: loaded.error }, actions]
  if (!listing) return [{ status: 'loading' }, actions]
  return [{ status: 'ready', entries, truncated: listing.truncated, loading, refreshing }, actions]
}
