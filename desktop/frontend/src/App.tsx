import type { CSSProperties } from 'react'
import { CentreBoundary } from './components/CentreBoundary'
import { AddProjectDialog } from './components/AddProjectDialog'
import { ConfirmHost } from './components/ConfirmHost'
import { UsageModal } from './components/UsageModal'
import { Divider } from './components/Divider'
import { Home } from './components/Home'
import { LeftSidebar } from './components/LeftSidebar'
import { PanelOverlay } from './components/PanelOverlay'
import { RightPanel } from './components/RightPanel'
import { WhatsNew } from './components/WhatsNew'
import { SettingsPanel } from './components/SettingsPanel'
import { ShortcutsSheet } from './components/ShortcutsSheet'
import { StartSplash } from './components/StartSplash'
import { Toasts } from './components/Toasts'
import { TopBar } from './components/TopBar'
import { Viewport } from './components/Viewport'
import { useOrchestrator } from './OrchestratorContext'
import { useConfirming } from './confirm'
import { useLayout } from './LayoutContext'
import { useFocusedPanel } from './useFocusedPanel'
import { useFullscreen } from './useFullscreen'
import { useDigestToasts } from './useDigestToasts'
import { useDockIcon } from './useDockIcon'
import { useNetwork } from './useNetwork'
import { useShortcuts } from './useShortcuts'

export function App() {
  useNetwork()
  const confirming = useConfirming()
  useShortcuts(!confirming)
  useDigestToasts()
  useFocusedPanel()
  useDockIcon()
  const fullscreen = useFullscreen()
  const { selectedId } = useOrchestrator('selectedId')
  const { mode, docked, sidebarOpen, rightOpen, overlay, panelExpanded } = useLayout()

  return (
    <>
      <div
        className="flex h-full flex-col bg-app"
        style={{ '--title-inset': fullscreen ? 'var(--corner-pad)' : 'var(--traffic-light-zone)' } as CSSProperties}
        data-mode={mode}
      >
        <div className="relative flex min-h-0 flex-1">
          {docked.sidebar && sidebarOpen && <LeftSidebar />}
          {docked.sidebar && sidebarOpen && <Divider panel="sidebar" />}
          {/* The expanded panel takes the thread's room; the thread stays mounted, so it comes back as it was. */}
          <div className={`flex min-w-0 flex-col ${panelExpanded ? 'w-0 flex-none overflow-hidden' : 'flex-1'}`} {...(panelExpanded ? { inert: '' } : {})}>
            <TopBar />
            <main className="relative min-h-0 min-w-0 flex-1">
              <CentreBoundary>{selectedId ? <Viewport /> : <Home />}</CentreBoundary>
              {overlay && <PanelOverlay panel={overlay} />}
            </main>
          </div>
          {docked.right && rightOpen && !panelExpanded && <Divider panel="right" />}
          {docked.right && rightOpen && <RightPanel />}
        </div>
        <AddProjectDialog />
        <ShortcutsSheet />
        <SettingsPanel />
        <WhatsNew />
        <Toasts />
      </div>
      <UsageModal />
      <ConfirmHost />
      <StartSplash />
    </>
  )
}
