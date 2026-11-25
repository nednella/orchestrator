import { useLayout } from '../LayoutContext'
import type { SidePanel } from '../LayoutContext'
import { LeftSidebar } from './LeftSidebar'
import { RightPanel } from './RightPanel'

type PanelOverlayProps = { panel: SidePanel }

// A side panel opened over the thread, when it has no room beside it. It covers the thread only: the top bar above
// it and the shell below it stay as they are, and a click on the thread beside it closes it.
export function PanelOverlay({ panel }: PanelOverlayProps) {
  const { closeOverlay } = useLayout()
  const left = panel === 'sidebar'
  return (
    <div className="absolute inset-0" style={{ zIndex: 'var(--z-panel-overlay)' }}>
      {/* Keeps the focus where it is until the click lands, so the overlay hands it back as its trigger does. */}
      <div className="absolute inset-0" onMouseDown={(e) => e.preventDefault()} onClick={() => closeOverlay('thread-focused')} />
      <div className={`side-overlay absolute inset-y-0 flex max-w-full overflow-hidden border-line ${left ? 'left-0 border-r' : 'right-0 border-l'}`}>
        {left ? <LeftSidebar /> : <RightPanel />}
      </div>
    </div>
  )
}
