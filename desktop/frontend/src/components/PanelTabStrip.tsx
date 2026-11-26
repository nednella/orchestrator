import { List, Maximize2, Minimize2, X } from 'lucide-react'
import { useLayout } from '../LayoutContext'
import { panelView } from '../panelViews'
import type { PanelViewId } from '../panelTabs'
import { titleBarButton } from './TitleBarButton'


const iconProps = { size: 14, strokeWidth: 2.25, 'aria-hidden': true } as const
const pressedColor = (pressed: boolean) => ({ color: pressed ? 'var(--text)' : undefined })

// PanelTabStrip heads the right panel: the list of views, a tab for each view open, marked while it waits on the owner,
// then Expand, offered only beside the chat, and Close. Its buttons are the top bar's, in the same box and at the same
// right edge, so the two rows read as one column.
export function PanelTabStrip() {
  const { panel, panelExpanded, docked, showPanelList, showPanelView, closePanelTab, togglePanelExpanded, togglePanel } = useLayout()
  const expand = panelExpanded ? 'Restore panel size' : 'Expand panel'

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
    <div className="flex h-[52px] flex-none items-center gap-1 border-b border-line pl-2" style={{ paddingRight: 'var(--corner-pad)' }}>
      <button className={titleBarButton} style={pressedColor(panel.active === null)} title="All views" aria-label="All views" aria-pressed={panel.active === null} onClick={showPanelList}>
        <List {...iconProps} />
      </button>
      <div role="tablist" aria-label="Open views" className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto [scrollbar-width:none]">
        {panel.tabs.map(tab)}
      </div>
      <div className="flex flex-none items-center gap-0.5">
        {docked.right && (
          <button className={titleBarButton} style={pressedColor(panelExpanded)} title={expand} aria-label={expand} aria-pressed={panelExpanded} onClick={togglePanelExpanded}>
            {panelExpanded ? <Minimize2 {...iconProps} /> : <Maximize2 {...iconProps} />}
          </button>
        )}
        <button className={titleBarButton} title="Close the panel" aria-label="Close the panel" onClick={() => togglePanel('right')}>
          <X {...iconProps} />
        </button>
      </div>
    </div>
  )
}
