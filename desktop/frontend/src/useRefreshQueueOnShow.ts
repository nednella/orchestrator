import { useEffect, useRef } from 'react'
import { useOrchestrator, useOrchestratorSelector } from './OrchestratorContext'

const MIN_GAP_MS = 5000

// Shared by every caller, so two of them on screen at once read the queue once.
let last = { at: 0, projectKey: undefined as string | undefined }

// Issues change on GitHub with no event the app hears, so issues on screen are read again when they are
// shown, when the project changes and when the window comes back into focus. The caller is mounted only
// while it shows issues.
export function useRefreshQueueOnShow() {
  const { refreshIssues, report } = useOrchestrator('refreshIssues', 'report')
  const projectKey = useOrchestratorSelector((a) => a.project?.key)
  const refresh = useRef<() => void>(() => {})
  refresh.current = () => {
    if (projectKey === undefined || (last.projectKey === projectKey && Date.now() - last.at < MIN_GAP_MS)) return
    last = { at: Date.now(), projectKey }
    report(refreshIssues)
  }

  useEffect(() => {
    refresh.current()
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
