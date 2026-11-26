import { useCallback, useEffect, useRef, useState } from 'react'
import { api, errorMessage } from './api'
import type { FolderListing } from './types'

type Read = { directory: string; listing: FolderListing | null; error: string }

// useFolderListing reads the folders in directory, null for none. A folder read once is kept while the popup is open.
// prefetch reads one ahead, so a list can be swapped for the next without a gap.
export function useFolderListing(directory: string | null) {
  const cache = useRef(new Map<string, FolderListing>())
  const [read, setRead] = useState<Read | null>(null)

  useEffect(() => {
    if (directory === null) return
    const hit = cache.current.get(directory)
    if (hit) {
      setRead({ directory, listing: hit, error: '' })
      return
    }
    let live = true
    api.listFolders(directory).then(
      (listing) => {
        cache.current.set(directory, listing)
        if (live) setRead({ directory, listing, error: '' })
      },
      (err) => live && setRead({ directory, listing: null, error: errorMessage(err) }),
    )
    return () => {
      live = false
    }
  }, [directory])

  const prefetch = useCallback(async (path: string) => {
    if (cache.current.has(path)) return
    try {
      cache.current.set(path, await api.listFolders(path))
    } catch {
      // the list shows the error once it is the one on screen
    }
  }, [])

  const current = directory !== null && read?.directory === directory ? read : null
  return { listing: current?.listing ?? null, error: current?.error ?? '', pending: directory !== null && current === null, prefetch }
}
