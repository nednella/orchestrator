import type { CSSProperties } from 'react'
import { useOrchestrator, useLatestOrchestrator } from '../OrchestratorContext'
import { actionsFor, withShortcut } from '../actions'
import { useLayout } from '../LayoutContext'
import { useDrawerOpen } from '../terminalDrawer'
import { ThreadCardToggle } from './ThreadCardToggle'
import { TitleBarButton } from './TitleBarButton'
import { TitleBreadcrumb } from './TitleBreadcrumb'

const drag = { '--wails-draggable': 'drag' } as CSSProperties
const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

export function TopBar() {
  const { selectedId, project, report } = useOrchestrator('selectedId', 'project', 'report')
  const layout = useLayout()
  const { docked, sidebarOpen, rightOpen, digestOpen, panelExpanded } = layout
  const terminalOpen = useDrawerOpen(selectedId ?? '') && !panelExpanded
  const latestOrchestrator = useLatestOrchestrator()
  const find = (id: string) => actionsFor(latestOrchestrator(), layout).find((a) => a.id === id)!
  const named = (label: string, id: string) => withShortcut(label, find(id).shortcut)
  const showsThread = selectedId !== null && !digestOpen
  // With no sidebar beside it the bar starts after the window buttons.
  const left = sidebarOpen && docked.sidebar ? '0px' : 'calc(var(--title-inset) - 0.75rem)'

  return (
    <header className="relative flex flex-none items-center" style={{ ...drag, height: 'var(--topbar-h)', paddingLeft: left, paddingRight: 'var(--corner-pad)' }}>
      <div className="flex min-w-0 flex-1 items-center px-3">
        <TitleBreadcrumb showsThread={showsThread} />
      </div>
      <div className="flex flex-none items-center gap-0.5" style={noDrag}>
        <ThreadCardToggle place="bar" />
        <TitleBarButton label={named(sidebarOpen ? 'Hide left sidebar' : 'Show left sidebar', 'toggle-sidebar')} icon="panel-left" pressed={sidebarOpen} onClick={() => find('toggle-sidebar').run()} />
        <TitleBarButton
          label={project ? named('Toggle terminal', 'toggle-shell') : 'The terminal is unavailable'}
          icon="panel-bottom"
          pressed={terminalOpen}
          disabled={!project}
          onClick={() => report(() => find('toggle-shell').run())}
        />
        <TitleBarButton
          label={named(rightOpen ? 'Hide right panel' : 'Show right panel', 'toggle-right')}
          icon="panel-right"
          pressed={rightOpen}
          onClick={() => report(() => find('toggle-right').run())}
        />
      </div>
    </header>
  )
}
