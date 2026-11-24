import type { CSSProperties } from 'react'
import { useLayout } from '../LayoutContext'
import { ProjectRail } from './ProjectRail'
import { ThreadSidebar } from './ThreadSidebar'

const drag = { '--wails-draggable': 'drag' } as CSSProperties

export function LeftSidebar() {
  const { mode } = useLayout()
  return (
    <div data-sidebar className={`flex min-h-0 flex-col ${mode === 'narrow' ? 'min-w-0 flex-1' : 'flex-none'}`}>
      <div className="flex flex-none items-center justify-center border-b border-line text-ink" style={{ ...drag, height: 'var(--topbar-h)' }}>
        <span className="text-title font-semibold tracking-wide">agentos</span>
      </div>
      <div className="flex min-h-0 flex-1">
        <ProjectRail />
        <ThreadSidebar />
      </div>
    </div>
  )
}
