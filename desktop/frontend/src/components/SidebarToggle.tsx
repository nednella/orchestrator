import type { CSSProperties } from 'react'
import { useLatestAgentos } from '../AgentosContext'
import { actionsFor, formatShortcut } from '../actions'
import { useLayout } from '../LayoutContext'
import { TitleBarButton } from './TitleBarButton'

const drag = { '--wails-draggable': 'drag' } as CSSProperties
const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

// Sits right after the window buttons, so it stays in one place whether the sidebar is shown or not.
export function SidebarToggle() {
  const layout = useLayout()
  const latestAgentos = useLatestAgentos()
  const action = actionsFor(latestAgentos(), layout).find((a) => a.id === 'toggle-sidebar')!
  const label = `${layout.sidebarOpen ? 'Hide' : 'Show'} left sidebar (${formatShortcut(action.shortcut!)})`
  return (
    <div className="absolute top-0 z-10 flex items-center" style={{ ...drag, left: 'var(--sidebar-toggle-left)', height: 'var(--topbar-h)' }}>
      <span className="flex" style={noDrag}>
        <TitleBarButton label={label} icon="panel-left" pressed={layout.sidebarOpen} onClick={() => action.run([])} />
      </span>
    </div>
  )
}
