import { PanelLeft, PanelRight } from 'lucide-react'
import type { CSSProperties } from 'react'
import { useOrchestrator, useLatestOrchestrator } from '../OrchestratorContext'
import { actionsFor, withShortcut } from '../actions'
import { useLayout } from '../LayoutContext'
import { TitleBarButton } from './TitleBarButton'

const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

// WindowButtons are the two toggles for the side panels, at the top right of the window.
export function WindowButtons() {
  const { report } = useOrchestrator('report')
  const layout = useLayout()
  const { sidebarOpen, rightOpen } = layout
  const latestOrchestrator = useLatestOrchestrator()
  const find = (id: string) => actionsFor(latestOrchestrator(), layout).find((a) => a.id === id)!
  const named = (label: string, id: string) => withShortcut(label, find(id).shortcut)

  return (
    <div className="flex flex-none items-center gap-0.5" style={noDrag}>
      <TitleBarButton label={named(sidebarOpen ? 'Hide left sidebar' : 'Show left sidebar', 'toggle-sidebar')} icon={PanelLeft} pressed={sidebarOpen} onClick={() => find('toggle-sidebar').run()} />
      <TitleBarButton
        label={named(rightOpen ? 'Hide right panel' : 'Show right panel', 'toggle-right')}
        icon={PanelRight}
        pressed={rightOpen}
        onClick={() => report(() => find('toggle-right').run())}
      />
    </div>
  )
}
