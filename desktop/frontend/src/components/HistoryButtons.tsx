import { ArrowLeft, ArrowRight } from 'lucide-react'
import type { CSSProperties } from 'react'
import { useAgentos, useLatestAgentos } from '../AgentosContext'
import { actionsFor, formatShortcut } from '../actions'
import { useLayout } from '../LayoutContext'
import { titleBarButton } from './TitleBarButton'

const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

// HistoryButtons are Back and Forward through the views the owner went through, off at either end.
export function HistoryButtons() {
  const { viewHistory, goBack, goForward } = useAgentos('viewHistory', 'goBack', 'goForward')
  const latestAgentos = useLatestAgentos()
  const layout = useLayout()
  const named = (label: string, id: string) => {
    const shortcut = actionsFor(latestAgentos(), layout).find((a) => a.id === id)?.shortcut
    return shortcut ? `${label} (${formatShortcut(shortcut)})` : label
  }

  return (
    <span className="flex flex-none items-center" style={noDrag}>
      <button className={titleBarButton} title={named('Back', 'history-back')} aria-label="Back" disabled={viewHistory.at <= 0} onClick={goBack}>
        <ArrowLeft size={14} strokeWidth={2.25} aria-hidden="true" />
      </button>
      <button className={titleBarButton} title={named('Forward', 'history-forward')} aria-label="Forward" disabled={viewHistory.at >= viewHistory.views.length - 1} onClick={goForward}>
        <ArrowRight size={14} strokeWidth={2.25} aria-hidden="true" />
      </button>
    </span>
  )
}
