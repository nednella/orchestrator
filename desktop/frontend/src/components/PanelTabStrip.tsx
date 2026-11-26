import { List, X } from 'lucide-react'
import { useAgentosSelector } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { agentsWait, panelView } from '../panelViews'
import type { PanelViewId } from '../panelTabs'
import type { Session } from '../types'

type PanelTabStripProps = { session: Session | undefined }

const stripButton = 'btn btn-ghost h-8 w-8 flex-none justify-center px-0 aria-pressed:bg-(--bg-segment-on) aria-pressed:text-ink'

// PanelTabStrip heads the right panel: the list of views, a tab for each view open, marked while it waits on the owner,
// then Close.
export function PanelTabStrip({ session }: PanelTabStripProps) {
  const { panel, showPanelList, showPanelView, closePanelTab, togglePanel } = useLayout()
  const agentsWaiting = useAgentosSelector((a) => agentsWait(session, a.sessions))

  const tab = (id: PanelViewId) => {
    const view = panelView(id)
    if (!view) return null
    const active = panel.active === id
    return (
      <div
        key={id}
        role="presentation"
        className={`group relative flex h-6 max-w-36 flex-none items-center rounded-md text-small ${active ? 'bg-(--bg-segment-on) text-ink' : 'text-soft hover:bg-(--bg-row-hover) hover:text-ink'}`}
        onMouseDown={(e) => e.button === 1 && e.preventDefault()}
        onAuxClick={(e) => e.button === 1 && closePanelTab(id)}
      >
        <button role="tab" aria-selected={active} className="flex h-full min-w-0 items-center rounded-md pr-2 pl-[1.375rem]" onClick={() => showPanelView(id)}>
          <span className="truncate">{view.label}</span>
        </button>
        <span className="pointer-events-none absolute top-1/2 left-1 grid size-4 -translate-y-1/2 place-items-center group-focus-within:opacity-0 group-hover:opacity-0">
          <view.icon size={12} aria-hidden="true" />
          {id === 'agents' && agentsWaiting && <span className="absolute right-0 bottom-0 size-1.5 rounded-full bg-(--waiting)" title="Waits on you" />}
        </span>
        <button
          className="absolute top-1/2 left-1 grid size-4 -translate-y-1/2 place-items-center rounded-sm text-soft opacity-0 group-focus-within:opacity-100 group-hover:opacity-100 hover:bg-(--bg-segment-on) hover:text-ink"
          aria-label={`Close ${view.label}`}
          onClick={() => closePanelTab(id)}
        >
          <X size={12} aria-hidden="true" />
        </button>
      </div>
    )
  }

  return (
    <div className="flex h-[52px] flex-none items-center gap-1 border-b border-line pr-2 pl-2">
      <button className={stripButton} title="All views" aria-label="All views" aria-pressed={panel.active === null} onClick={showPanelList}>
        <List size={16} aria-hidden="true" />
      </button>
      <div role="tablist" aria-label="Open views" className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto [scrollbar-width:none]">
        {panel.tabs.map(tab)}
      </div>
      <button className={stripButton} title="Close the panel" aria-label="Close the panel" onClick={() => togglePanel('right')}>
        <X size={16} aria-hidden="true" />
      </button>
    </div>
  )
}
