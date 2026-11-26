import { useRef } from 'react'
import { useOrchestratorSelector } from '../OrchestratorContext'
import { useLayout } from '../LayoutContext'
import { panelView } from '../panelViews'
import { useFocusRequest } from '../useFocusRequest'
import { PanelHome } from './PanelHome'
import { PanelTabStrip } from './PanelTabStrip'

// RightPanel shows the open thread's views, or Home's: the list of views, or the tab picked, which fills the panel.
// Expanded, it takes the chat's room as well.
export function RightPanel() {
  const session = useOrchestratorSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const projectKey = useOrchestratorSelector((a) => a.project?.key)
  const { widths, panel, panelExpanded } = useLayout()
  const view = panel.active ? panelView(panel.active) : undefined
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
      <PanelTabStrip />
      <div className="@container relative min-h-0 flex-1">{view ? <view.Component key={`${projectKey} ${view.id}`} session={session} /> : <PanelHome session={session} />}</div>
    </aside>
  )
}
