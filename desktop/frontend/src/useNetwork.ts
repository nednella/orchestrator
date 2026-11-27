import { useCallback, useEffect, useRef } from 'react'
import { useLatestOrchestrator, useOrchestrator } from './OrchestratorContext'
import { on } from './api'

const NETWORK_TOAST = 'network'

// The app says it is offline when its GitHub and git calls get no answer, and online at the next answer.
// The web view's own offline signal is left out: no answer from the app's calls could end it.
// Offline shows as one toast that stays until the network is back or the owner closes it.
export function useNetwork() {
  const { refreshIssues, refreshPRs, pushToast, dismissToast } = useOrchestrator('refreshIssues', 'refreshPRs', 'pushToast', 'dismissToast')
  const latest = useLatestOrchestrator()
  const online = useRef(true)

  const shown = useCallback(() => latest().toasts.find((t) => t.id === NETWORK_TOAST), [latest])

  const announce = useCallback(
    (retry: () => Promise<void>, busy: boolean) =>
      pushToast({
        id: NETWORK_TOAST,
        tone: 'network',
        sticky: true,
        busy,
        text: busy ? 'Retrying…' : "Can't reach GitHub",
        detail: busy ? undefined : 'Issues and pull requests refresh when it is back.',
        action: busy ? undefined : { label: 'Retry', run: () => void retry() },
      }),
    [pushToast],
  )

  const retry = useCallback(
    async function retryNow(): Promise<void> {
      if (shown()) announce(retryNow, true)
      await Promise.allSettled([refreshIssues(), refreshPRs()])
      if (!online.current && shown()) announce(retryNow, false)
    },
    [shown, announce, refreshIssues, refreshPRs],
  )

  useEffect(
    () =>
      on('network', (status) => {
        const was = online.current
        online.current = status.online
        if (status.online) {
          const toast = shown()
          if (toast) dismissToast(toast.key)
        } else if (was) {
          announce(retry, false)
        }
      }),
    [shown, announce, retry, dismissToast],
  )
}
