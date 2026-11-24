import { useAgentosSelector } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { panelViewFor, shownPanelViews, usePanelContext } from '../panelViews'
import { Icon } from './Icon'

export function RightPanel() {
  const ctx = usePanelContext()
  const projectKey = useAgentosSelector((a) => a.project?.key)
  const { widths, setRightOpen, mode, panelView, setPanelView } = useLayout()
  const narrow = mode === 'narrow'
  const view = panelViewFor(panelView, ctx)

  return (
    <aside data-panel="right" tabIndex={-1} className={`panel flex h-full min-h-0 flex-col overflow-hidden ${narrow ? 'min-w-0 flex-1' : 'flex-none'}`} style={narrow ? undefined : { width: `${widths.right}rem` }}>
      {!narrow && (
        <div className="flex h-10 flex-none items-center gap-1 border-b border-line px-2">
          <div role="tablist" aria-label="Right panel" className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
            {shownPanelViews(ctx).map((tab) => {
              const count = tab.count?.(ctx)
              return (
                <button
                  key={tab.id}
                  role="tab"
                  aria-selected={tab.id === view.id}
                  className="flex h-7 flex-none items-center gap-1.5 rounded-md px-2.5 text-body font-medium text-soft hover:text-ink aria-selected:bg-hover aria-selected:text-ink"
                  onClick={() => setPanelView(tab.id)}
                >
                  {tab.label}
                  {count !== undefined && <span className="mono text-small text-dim">{count}</span>}
                </button>
              )
            })}
          </div>
          <button className="btn btn-ghost h-7 w-7 flex-none justify-center px-0" title="Close the right panel" aria-label="Close the right panel" onClick={() => setRightOpen(false)}>
            <Icon name="close" />
          </button>
        </div>
      )}
      <div className="relative min-h-0 flex-1">
        <view.Component key={projectKey} ctx={ctx} />
      </div>
    </aside>
  )
}
