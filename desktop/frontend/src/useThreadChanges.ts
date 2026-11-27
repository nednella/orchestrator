import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from './api'
import { turnEnded } from './serversRead'
import type { Changes, State } from './types'

// useThreadChanges reads the lines thread id changes against origin's default branch when the caller mounts and at each
// turn end of the thread, null until the first read and when there is nothing to compare with. Only the latest read
// counts; a read that fails shows nothing.
export function useThreadChanges(id: string, state: State | undefined): Changes | null {
  const [found, setFound] = useState<{ id: string; changes: Changes | null }>({ id, changes: null })
  const reads = useRef(0)
  const refresh = useCallback(async () => {
    const read = ++reads.current
    const changes = await api.changes(id).catch((): null => null)
    if (read === reads.current) setFound({ id, changes })
  }, [id])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const last = useRef({ id, state })
  useEffect(() => {
    const prev = last.current
    last.current = { id, state }
    if (prev.id === id && turnEnded(prev.state, state)) void refresh()
  }, [id, state, refresh])

  return found.id === id ? found.changes : null
}
