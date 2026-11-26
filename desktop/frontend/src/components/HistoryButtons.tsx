import { ArrowLeft, ArrowRight } from 'lucide-react'
import { useState } from 'react'
import type { CSSProperties } from 'react'
import { useAgentos, useLatestAgentos } from '../AgentosContext'
import { actionsFor, withShortcut } from '../actions'
import { useLayout } from '../LayoutContext'
import { titleBarButton } from './TitleBarButton'

const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

// HistoryButtons are Back and Forward through the views the owner went through, each off while no view is left that
// way. The shortcuts never change, so the labels are read from the actions once.
export function HistoryButtons() {
  const { canGoBack, canGoForward, goBack, goForward } = useAgentos('canGoBack', 'canGoForward', 'goBack', 'goForward')
  const latestAgentos = useLatestAgentos()
  const layout = useLayout()
  const [labels] = useState(() => {
    const actions = actionsFor(latestAgentos(), layout)
    const shortcut = (id: string) => actions.find((a) => a.id === id)?.shortcut
    return { back: withShortcut('Back', shortcut('history-back')), forward: withShortcut('Forward', shortcut('history-forward')) }
  })

  return (
    <span className="flex flex-none items-center" style={noDrag}>
      <button className={titleBarButton} title={labels.back} aria-label="Back" disabled={!canGoBack} onClick={goBack}>
        <ArrowLeft size={14} strokeWidth={2.25} aria-hidden="true" />
      </button>
      <button className={titleBarButton} title={labels.forward} aria-label="Forward" disabled={!canGoForward} onClick={goForward}>
        <ArrowRight size={14} strokeWidth={2.25} aria-hidden="true" />
      </button>
    </span>
  )
}
