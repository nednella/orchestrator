import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from './api'
import type { Server } from './types'

const POLL_MS = 5000

// The threads whose details show servers now, for [ and ] to step to their Servers tab.
const serving = new Set<string>()

// serversTabShown is whether the thread's details show its Servers tab for the servers it has.
export function serversTabShown(thread: string): boolean {
  return serving.has(thread)
}

// useThreadServers reads the servers of a running thread now and every few seconds while the caller is mounted, and
// not at all for a settled thread, which has none. Only the latest read counts. A read that fails leaves the list
// empty: the thread may just have been deleted.
export function useThreadServers(id: string, live: boolean): { servers: Server[]; refresh(): Promise<void> } {
  const [found, setFound] = useState<{ id: string; servers: Server[] }>({ id, servers: [] })
  const reads = useRef(0)
  const refresh = useCallback(async () => {
    const read = ++reads.current
    const servers = live ? await api.servers(id).catch((): Server[] => []) : []
    if (read === reads.current) setFound({ id, servers })
  }, [id, live])

  useEffect(() => {
    void refresh()
    if (!live) return
    const timer = window.setInterval(() => void refresh(), POLL_MS)
    return () => window.clearInterval(timer)
  }, [refresh, live])

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
