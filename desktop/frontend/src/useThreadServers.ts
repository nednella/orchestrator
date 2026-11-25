import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from './api'
import { pollsServers, SERVERS_POLL_MS, turnEnded } from './serversRead'
import type { Server, State } from './types'

// The threads whose details show servers now, for [ and ] to step to their Servers tab.
const serving = new Set<string>()

// serversTabShown is whether the thread's details show its Servers tab for the servers it has.
export function serversTabShown(thread: string): boolean {
  return serving.has(thread)
}

// useThreadServers reads the servers of thread id when the caller mounts, at each turn end of the thread, and every few
// seconds while their tab is shown; a settled thread has none. Only the latest read counts. A read that fails leaves
// the list empty: the thread may just have been deleted.
export function useThreadServers(id: string, state: State | undefined, shown: boolean): { servers: Server[]; refresh(): Promise<void> } {
  const [found, setFound] = useState<{ id: string; servers: Server[] }>({ id, servers: [] })
  const reads = useRef(0)
  const live = state !== undefined && state !== 'settled'
  const refresh = useCallback(async () => {
    const read = ++reads.current
    const servers = live ? await api.servers(id).catch((): Server[] => []) : []
    if (read === reads.current) setFound({ id, servers })
  }, [id, live])

  const polls = pollsServers(shown, state)
  useEffect(() => {
    void refresh()
    if (!polls) return
    const timer = window.setInterval(() => void refresh(), SERVERS_POLL_MS)
    return () => window.clearInterval(timer)
  }, [refresh, polls])

  const last = useRef({ id, state })
  useEffect(() => {
    const prev = last.current
    last.current = { id, state }
    if (prev.id === id && turnEnded(prev.state, state)) void refresh()
  }, [id, state, refresh])

  const servers = found.id === id ? found.servers : []
  const has = servers.length > 0
  useEffect(() => {
    if (!has) return
    serving.add(id)
    return () => {
      serving.delete(id)
    }
  }, [id, has])

  return { servers, refresh }
}
