import { SquareMenu } from 'lucide-react'
import { useRef } from 'react'
import type { CSSProperties } from 'react'
import { useAgentos, useAgentosSelector, useLatestAgentos } from '../AgentosContext'
import { actionsFor, withShortcut } from '../actions'
import { useCardPopoverDismiss, useContextCard } from '../contextCardOpen'
import { useLayout } from '../LayoutContext'
import { useDrawerOpen } from '../terminalDrawer'
import { selectedCardHasRows } from '../useCardFacts'
import { ThreadContextCard } from './ThreadContextCard'
import { TitleBarButton, titleBarButton } from './TitleBarButton'
import { TitleBreadcrumb } from './TitleBreadcrumb'

const drag = { '--wails-draggable': 'drag' } as CSSProperties
const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

export function TopBar() {
  const { selectedId, project, report } = useAgentos('selectedId', 'project', 'report')
  const layout = useLayout()
  const { docked, sidebarOpen, rightOpen, statsOpen, digestOpen, panelExpanded } = layout
  const terminalOpen = useDrawerOpen(selectedId ?? '') && !panelExpanded
  const latestAgentos = useLatestAgentos()
  const cardHasRows = useAgentosSelector(selectedCardHasRows)
  const card = useContextCard()
  const cardToggle = useRef<HTMLDivElement>(null)
  const popover = card.placement === 'popover' && card.popover
  const cardPressed = card.placement === 'inline' ? card.shown : popover
  useCardPopoverDismiss(cardToggle, popover)
  const find = (id: string) => actionsFor(latestAgentos(), layout).find((a) => a.id === id)!
  const named = (label: string, id: string) => withShortcut(label, find(id).shortcut)
  const showsThread = selectedId !== null && !statsOpen && !digestOpen
  // With no sidebar beside it the bar starts after the window buttons.
  const left = sidebarOpen && docked.sidebar ? '0px' : 'calc(var(--title-inset) - 0.75rem)'

  return (
    <header className="relative flex flex-none items-center" style={{ ...drag, height: 'var(--topbar-h)', paddingLeft: left, paddingRight: 'var(--corner-pad)' }}>
      <div className="flex min-w-0 flex-1 items-center px-3">
        <TitleBreadcrumb showsThread={showsThread} />
      </div>
      <div className="flex flex-none items-center gap-0.5" style={noDrag}>
        {showsThread && cardHasRows && (
          <div ref={cardToggle} className="relative flex">
            <button
              className={titleBarButton}
              style={{ color: cardPressed ? 'var(--text)' : undefined }}
              title={named('Toggle thread details', 'toggle-context-card')}
              aria-label={named('Toggle thread details', 'toggle-context-card')}
              aria-pressed={cardPressed}
              onClick={() => report(() => find('toggle-context-card').run([]))}
            >
              <SquareMenu size={14} strokeWidth={2.25} aria-hidden="true" />
            </button>
            {popover && <ThreadContextCard id={selectedId} className="absolute top-full right-0 z-(--z-popup) mt-1 max-h-[calc(100vh-var(--topbar-h)-1rem)]" />}
          </div>
        )}
        <TitleBarButton label={named(sidebarOpen ? 'Hide left sidebar' : 'Show left sidebar', 'toggle-sidebar')} icon="panel-left" pressed={sidebarOpen} onClick={() => find('toggle-sidebar').run([])} />
        <TitleBarButton
          label={project ? named('Toggle terminal', 'toggle-shell') : 'The terminal is unavailable'}
          icon="panel-bottom"
          pressed={terminalOpen}
          disabled={!project}
          onClick={() => report(() => find('toggle-shell').run([]))}
        />
        <TitleBarButton
          label={named(rightOpen ? 'Hide right panel' : 'Show right panel', 'toggle-right')}
          icon="panel-right"
          pressed={rightOpen}
          onClick={() => report(() => find('toggle-right').run([]))}
        />
      </div>
    </header>
  )
}
