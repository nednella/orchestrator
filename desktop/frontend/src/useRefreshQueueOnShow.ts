import { useEffect, useRef } from 'react'
import { useAgentos, useAgentosSelector } from './AgentosContext'

const MIN_GAP_MS = 5000

// Issues change on GitHub with no event the app hears, so a queue on screen is read again when Home is
// shown, when the project changes and when the window comes back into focus. The caller is mounted only
// while Home is on screen.
export function useRefreshQueueOnShow() {
  const { refreshIssues, report } = useAgentos('refreshIssues', 'report')
  const projectKey = useAgentosSelector((a) => a.project?.key)
  const refresh = useRef<(force?: boolean) => void>(() => {})
  const last = useRef(Date.now())
  refresh.current = (force = false) => {
    if (!force && Date.now() - last.current < MIN_GAP_MS) return
    last.current = Date.now()
    report(refreshIssues)
  }

  const seen = useRef(projectKey)
  useEffect(() => {
    const switched = seen.current !== undefined && seen.current !== projectKey
    seen.current = projectKey
    refresh.current(switched)
  }, [projectKey])

  useEffect(() => {
    const onFocus = () => document.visibilityState === 'visible' && refresh.current()
    window.addEventListener('focus', onFocus)
    document.addEventListener('visibilitychange', onFocus)
    return () => {
      window.removeEventListener('focus', onFocus)
      document.removeEventListener('visibilitychange', onFocus)
    }
  }, [])
}
