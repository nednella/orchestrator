import { List, Maximize2, MessagesSquare, Minimize2, SquareTerminal, X } from 'lucide-react'
import { useLayout } from '../LayoutContext'
import { useOrchestrator } from '../OrchestratorContext'
import { panelView } from '../panelViews'
import { sideChatOf, viewOfTab } from '../panelTabs'
import type { PanelTabId } from '../panelTabs'
import { SIDE_CHAT_TITLE, sideChatDot } from '../sideChat'
import { useSideChat } from '../useSideChat'
import { closeTab, labelOf, selectTab } from '../projectTerminalOps'
import { folderName } from '../terminalGroups'
import type { TermGroup } from '../terminalGroups'
import { useProjectTerminals } from '../useProjectTerminals'
import { PanelTab } from './PanelTab'
import { titleBarButton } from './TitleBarButton'


const iconProps = { size: 14, strokeWidth: 2.25, 'aria-hidden': true } as const
const pressedColor = (pressed: boolean) => ({ color: pressed ? 'var(--text)' : undefined })

// PanelTabStrip heads the right panel: the list of views, a tab for each view open and one for each group of the
// project's terminals, then Expand, offered only beside the chat, and Close. Its buttons are the top bar's, in the same box and at the same
// right edge, so the two rows read as one column.
export function PanelTabStrip() {
  const { report, sideChats } = useOrchestrator('report', 'sideChats')
  const { terms, shells } = useProjectTerminals()
  const { panel, panelExpanded, docked, showPanelList, showPanelView, closePanelTab, togglePanelExpanded, togglePanel } = useLayout()
  const sideChat = useSideChat()
  const expand = panelExpanded ? 'Restore panel size' : 'Expand panel'

  const viewTab = (id: PanelTabId) => {
    const side = sideChatOf(id)
    if (side !== null) {
      const s = sideChats.find((x) => x.id === side)
      return <PanelTab key={id} label={s?.title ?? SIDE_CHAT_TITLE} icon={MessagesSquare} dot={s && sideChatDot(s.state)} active={panel.active === id} onSelect={() => showPanelView(id)} onClose={() => sideChat.close(side)} />
    }
    const shown = viewOfTab(id)
    const view = shown && panelView(shown)
    if (!view) return null
    return <PanelTab key={id} label={view.label} icon={view.icon} active={panel.active === id} onSelect={() => showPanelView(id)} onClose={() => closePanelTab(id)} />
  }

  const terminalTab = (g: TermGroup) => {
    const shell = shells.find((s) => s.id === g.active)
    const label = labelOf(shells, g.active)
    return (
      <PanelTab
        key={g.id}
        label={label}
        note={shell?.folder ? folderName(shell.folder) : undefined}
        tip={shell?.folder || undefined}
        icon={SquareTerminal}
        active={panel.active === 'terminal' && terms.active === g.id}
        onSelect={() => {
          selectTab(g.id)
          showPanelView('terminal')
        }}
        onClose={() => report(() => closeTab(shells, terms, g.id))}
      />
    )
  }

  return (
    <div className="flex h-[52px] flex-none items-center gap-1 border-b border-line pl-2" style={{ paddingRight: 'var(--corner-pad)' }}>
      <button className={titleBarButton} style={pressedColor(panel.active === null)} title="All views" aria-label="All views" aria-pressed={panel.active === null} onClick={showPanelList}>
        <List {...iconProps} />
      </button>
      <div role="tablist" aria-label="Open views" className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto [scrollbar-width:none]">
        {panel.tabs.map(viewTab)}
        {terms.groups.map(terminalTab)}
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
