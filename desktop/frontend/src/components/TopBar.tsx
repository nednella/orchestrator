import type { CSSProperties } from 'react'
import { useLayout } from '../LayoutContext'
import { ThreadCardToggle } from './ThreadCardToggle'
import { TitleBreadcrumb } from './TitleBreadcrumb'
import { WindowButtons } from './WindowButtons'

const drag = { '--wails-draggable': 'drag' } as CSSProperties
const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

// TopBar is the row over the thread. Beside the right panel the panel's column has the window buttons, so the bar ends
// at the thread's edge with the thread details button.
export function TopBar() {
  const { docked, sidebarOpen, rightOpen } = useLayout()
  const besidePanel = docked.right && rightOpen
  // With no sidebar beside it the bar starts after the window buttons.
  const left = sidebarOpen && docked.sidebar ? '0px' : 'calc(var(--title-inset) - 0.75rem)'

  return (
    <header className="relative flex flex-none items-center" style={{ ...drag, height: 'var(--topbar-h)', paddingLeft: left, paddingRight: besidePanel ? '0.75rem' : 'var(--corner-pad)' }}>
      <div className="flex min-w-0 flex-1 items-center px-3">
        <TitleBreadcrumb />
      </div>
      <div className="flex flex-none items-center gap-0.5" style={noDrag}>
        <ThreadCardToggle />
        {!besidePanel && <WindowButtons />}
      </div>
    </header>
  )
}
