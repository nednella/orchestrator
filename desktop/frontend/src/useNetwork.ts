import { useCallback, useEffect, useRef, useState } from 'react'
import { useLatestOrchestrator, useOrchestrator } from './OrchestratorContext'
import { on } from './api'
import type { NetworkStatus } from './NetworkContext'

const NETWORK_TOAST = 'network'

// The app says it is offline when its GitHub and git calls get no answer, and online at the next answer.
// The web view's own offline signal is left out: no answer from the app's calls could end it.
// Offline shows as one toast that stays until the network is back or the owner closes it; a closed toast stays closed
// until the owner retries.
export function useNetwork(): NetworkStatus {
  const { refreshIssues, refreshPRs, pushToast, dismissToast } = useOrchestrator('refreshIssues', 'refreshPRs', 'pushToast', 'dismissToast')
  const latest = useLatestOrchestrator()
  const [online, setOnline] = useState(true)
  const [retrying, setRetrying] = useState(false)
  const [offlineSince, setOfflineSince] = useState(0)
  const isOnline = useRef(true)

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
    async function retryNow(showToast = false): Promise<void> {
      if (showToast || shown()) announce(retryNow, true)
      setRetrying(true)
      await Promise.allSettled([refreshIssues(), refreshPRs()])
      setRetrying(false)
      if (!isOnline.current && shown()) announce(retryNow, false)
    },
    [shown, announce, refreshIssues, refreshPRs],
  )

  useEffect(
    () =>
      on('network', (status) => {
        const was = isOnline.current
        isOnline.current = status.online
        setOnline(status.online)
        if (status.online) {
          const toast = shown()
          if (toast) dismissToast(toast.key)
        } else if (was) {
          setOfflineSince(Date.now())
          announce(retry, false)
        }
      }),
    [shown, announce, retry, dismissToast],
  )

  return { online, retrying, offlineSince, retry: () => void retry(true) }
}
