import { useEffect, useRef } from 'react'
import { useOrchestrator } from './OrchestratorContext'
import { useLayout } from './LayoutContext'

export function useDigestToasts() {
  const { digest, pushToast, dismissToast } = useOrchestrator('digest', 'pushToast', 'dismissToast')
  const { panel, rightOpen, showPanelView } = useLayout()
  const newsShown = rightOpen && panel.active === 'news'
  const latest = useRef({ newsShown, pushToast, dismissToast, showPanelView })
  latest.current = { newsShown, pushToast, dismissToast, showPanelView }
  const previousDigest = useRef(digest)

  useEffect(() => {
    const before = previousDigest.current
    previousDigest.current = digest
    if (!before?.running || !digest || digest.running || before.project !== digest.project) return
    const seen = Math.max(0, ...before.items.map((i) => i.at))
    const fresh = digest.items.filter((i) => !i.noteId && i.at > seen).length
    if (fresh === 0 || latest.current.newsShown) return
    const { pushToast, dismissToast, showPanelView } = latest.current
    const key = pushToast({
      tone: 'info',
      text: `News: ${fresh} new ${fresh === 1 ? 'item' : 'items'}`,
      action: {
        label: 'Open',
        run() {
          showPanelView('news')
          dismissToast(key)
        },
      },
    })
  }, [digest])
}
