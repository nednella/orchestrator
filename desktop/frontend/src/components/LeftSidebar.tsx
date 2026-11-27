import type { CSSProperties } from 'react'
import { useLayout } from '../LayoutContext'
import { HistoryButtons } from './HistoryButtons'
import { ProjectRail } from './ProjectRail'
import { SidebarBottomBar } from './SidebarBottomBar'
import { ThreadSidebar } from './ThreadSidebar'

const drag = { '--wails-draggable': 'drag' } as CSSProperties

export function LeftSidebar() {
  const { docked } = useLayout()
  return (
    <div data-sidebar className="flex min-h-0 flex-none flex-col">
      {/* Centred in the strip, but pushed right of the window buttons, then cut short, when the sidebar is too narrow.
          Over the thread the sidebar starts under the top bar, which has the window buttons. It never widens the sidebar:
          the width is the rail's and the thread list's, so the line beside the sidebar stays on its edge. */}
      {docked.sidebar && (
        <div className="flex flex-none items-center border-b border-line text-ink [contain:inline-size]" style={{ ...drag, height: 'var(--topbar-h)' }}>
          <span className="flex-1" style={{ minWidth: 'var(--title-inset)' }} />
          <span className="min-w-0 truncate font-brand text-title font-bold">
            orchestrator<span className="text-[1.4em] leading-none font-black text-accent">.</span>
          </span>
          <span className="flex flex-1 justify-end pr-1.5">
            <HistoryButtons />
          </span>
        </div>
      )}
      <div className="flex min-h-0 flex-1">
        <ProjectRail />
        {/* The grid row stretches the thread list down to the bar, whose width it follows. */}
        <div className="grid min-h-0 grid-rows-[minmax(0,1fr)_auto]">
          <ThreadSidebar />
          <SidebarBottomBar />
        </div>
      </div>
    </div>
  )
}
