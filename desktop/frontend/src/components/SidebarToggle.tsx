import type { CSSProperties } from 'react'
import { useLatestAgentos } from '../AgentosContext'
import { actionsFor, formatShortcut } from '../actions'
import { useLayout } from '../LayoutContext'
import { useFullscreen } from '../useFullscreen'
import { TitleBarButton } from './TitleBarButton'

const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

// Stays just after the window buttons whether the thread list shows or not, as T3 Code's does.
export function SidebarToggle() {
  const layout = useLayout()
  const latestAgentos = useLatestAgentos()
  const fullscreen = useFullscreen()
  const action = actionsFor(latestAgentos(), layout).find((a) => a.id === 'toggle-sidebar')!
  const label = layout.sidebarOpen ? 'Hide thread list' : 'Show thread list'

  return (
    <div
      className="absolute top-0 flex items-center"
      style={{ ...noDrag, left: fullscreen ? 'var(--corner-pad)' : 'var(--traffic-light-zone)', height: 'var(--topbar-h)' }}
    >
      <TitleBarButton label={`${label} (${formatShortcut(action.shortcut!)})`} icon="panel-left" pressed={layout.sidebarOpen} onClick={() => action.run([])} />
    </div>
  )
}
