import { useEffect, useRef } from 'react'
import { useOrchestrator } from './OrchestratorContext'
import { useLayout } from './LayoutContext'

export function useDigestToasts() {
  const { digest, pushToast, dismissToast } = useOrchestrator('digest', 'pushToast', 'dismissToast')
  const { digestOpen, openDigest } = useLayout()
  const latest = useRef({ digestOpen, pushToast, dismissToast, openDigest })
  latest.current = { digestOpen, pushToast, dismissToast, openDigest }
  const previousDigest = useRef(digest)

  useEffect(() => {
    const before = previousDigest.current
    previousDigest.current = digest
    if (!before?.running || !digest || digest.running || before.project !== digest.project) return
    const seen = Math.max(0, ...before.items.map((i) => i.at))
    const fresh = digest.items.filter((i) => !i.noteId && i.at > seen).length
    if (fresh === 0 || latest.current.digestOpen) return
    const { pushToast, dismissToast, openDigest } = latest.current
    const key = pushToast({
      tone: 'info',
      text: `News: ${fresh} new ${fresh === 1 ? 'item' : 'items'}`,
      action: {
        label: 'Open',
        run() {
          openDigest()
          dismissToast(key)
        },
      },
    })
  }, [digest])
}
