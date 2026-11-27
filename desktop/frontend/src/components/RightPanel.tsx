import { useRef } from 'react'
import type { CSSProperties } from 'react'
import { useOrchestratorSelector } from '../OrchestratorContext'
import { useLayout } from '../LayoutContext'
import { sideChatOf, viewOfTab } from '../panelTabs'
import { panelView } from '../panelViews'
import { useFocusRequest } from '../useFocusRequest'
import { PanelHome } from './PanelHome'
import { PanelTabStrip } from './PanelTabStrip'
import { SideChatPane } from './SideChatPane'
import { WindowButtons } from './WindowButtons'

const drag = { '--wails-draggable': 'drag' } as CSSProperties

// RightPanel shows the open thread's views, or Home's: the list of views, or the tab picked, which fills the panel.
// Expanded, it takes the chat's room as well. Docked, its first row is the top bar's, with the window buttons, over one line.
export function RightPanel() {
  const session = useOrchestratorSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const projectKey = useOrchestratorSelector((a) => a.project?.key)
  const { widths, panel, panelExpanded, docked } = useLayout()
  const sideChat = sideChatOf(panel.active)
  const shown = viewOfTab(panel.active)
  const view = shown ? panelView(shown) : undefined
  const root = useRef<HTMLElement>(null)

  useFocusRequest('right', () => (root.current?.querySelector<HTMLElement>('[role="tab"][aria-selected="true"], [data-panel-view]:not([aria-disabled="true"])') ?? root.current)?.focus())

  return (
    <aside
      ref={root}
      data-panel="right"
      tabIndex={-1}
      className={`panel flex h-full min-h-0 flex-col overflow-hidden ${panelExpanded ? 'min-w-0 flex-1' : 'flex-none'}`}
      style={panelExpanded ? undefined : { width: `${widths.right}rem` }}
    >
      {docked.right && (
        <div className="flex flex-none items-center justify-end border-b border-line" style={{ ...drag, height: 'var(--topbar-h)', paddingRight: 'var(--corner-pad)' }}>
          <WindowButtons />
        </div>
      )}
      <PanelTabStrip />
      <div className="@container relative min-h-0 flex-1">
        {sideChat !== null && session ? <SideChatPane key={sideChat} id={sideChat} /> : view ? <view.Component key={`${projectKey} ${view.id}`} session={session} /> : <PanelHome session={session} />}
      </div>
    </aside>
  )
}
