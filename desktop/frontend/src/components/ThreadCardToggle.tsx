import { SquareMenu } from 'lucide-react'
import { useRef } from 'react'
import { useOrchestrator, useOrchestratorSelector, useLatestOrchestrator } from '../OrchestratorContext'
import { actionsFor, withShortcut } from '../actions'
import { useCardPopoverDismiss, useContextCard } from '../contextCardOpen'
import { useLayout } from '../LayoutContext'
import { selectedCardHasRows } from '../useCardFacts'
import { ThreadContextCard } from './ThreadContextCard'
import { titleBarButton } from './TitleBarButton'

// ThreadCardToggle is the button that shows the thread's context card, in the top bar.
export function ThreadCardToggle() {
  const { selectedId, report } = useOrchestrator('selectedId', 'report')
  const layout = useLayout()
  const latestOrchestrator = useLatestOrchestrator()
  const hasRows = useOrchestratorSelector(selectedCardHasRows)
  const card = useContextCard()
  const root = useRef<HTMLDivElement>(null)
  const popover = card.placement === 'popover' && card.popover
  useCardPopoverDismiss(root, popover)
  if (selectedId === null || !hasRows) return null

  const toggle = actionsFor(latestOrchestrator(), layout).find((a) => a.id === 'toggle-context-card')!
  const label = withShortcut('Toggle thread details', toggle.shortcut)
  const pressed = card.placement === 'inline' ? card.shown : popover
  return (
    <div ref={root} className="relative flex">
      <button className={titleBarButton} style={{ color: pressed ? 'var(--text)' : undefined }} title={label} aria-label={label} aria-pressed={pressed} onClick={() => report(() => toggle.run())}>
        <SquareMenu size={14} strokeWidth={2.25} aria-hidden="true" />
      </button>
      {popover && <ThreadContextCard id={selectedId} className="absolute top-full right-0 z-(--z-popup) mt-1 max-h-[calc(100vh-var(--topbar-h)-1rem)]" />}
    </div>
  )
}
