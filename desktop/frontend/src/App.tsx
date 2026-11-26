import type { CSSProperties } from 'react'
import { ConfirmHost } from './components/ConfirmHost'
import { UsageModal } from './components/UsageModal'
import { DigestView } from './components/DigestView'
import { Divider } from './components/Divider'
import { Home } from './components/Home'
import { LeftSidebar } from './components/LeftSidebar'
import { OfflineSplash } from './components/OfflineSplash'
import { PanelOverlay } from './components/PanelOverlay'
import { RightPanel } from './components/RightPanel'
import { PatchNotes } from './components/PatchNotes'
import { SettingsPanel } from './components/SettingsPanel'
import { ShortcutsSheet } from './components/ShortcutsSheet'
import { Stats } from './components/Stats'
import { TerminalDrawer } from './components/TerminalDrawer'
import { Toasts } from './components/Toasts'
import { TopBar } from './components/TopBar'
import { Viewport } from './components/Viewport'
import { useOrchestrator } from './OrchestratorContext'
import { useConfirming } from './confirm'
import { useLayout } from './LayoutContext'
import { useFocusedPanel } from './useFocusedPanel'
import { useFullscreen } from './useFullscreen'
import { useDigestToasts } from './useDigestToasts'
import { useNetwork } from './useNetwork'
import { useShortcuts } from './useShortcuts'

export function App() {
  const network = useNetwork()
  const confirming = useConfirming()
  useShortcuts(network.online && !confirming)
  useDigestToasts()
  useFocusedPanel()
  const fullscreen = useFullscreen()
  const { selectedId } = useOrchestrator('selectedId')
  const { mode, docked, sidebarOpen, rightOpen, overlay, statsOpen, digestOpen, panelExpanded } = useLayout()

  return (
    <>
      <div
        className="flex h-full flex-col bg-app"
        style={{ '--title-inset': fullscreen ? 'var(--corner-pad)' : 'var(--traffic-light-zone)' } as CSSProperties}
        data-mode={mode}
        {...(network.online ? {} : { inert: '' })}
      >
        <div className="relative flex min-h-0 flex-1">
          {docked.sidebar && sidebarOpen && <LeftSidebar />}
          {docked.sidebar && sidebarOpen && <Divider panel="sidebar" />}
          <div className="flex min-w-0 flex-1 flex-col">
            <TopBar />
            <main className="relative flex min-h-0 min-w-0 flex-1">
              {/* The expanded panel takes the chat's room; the chat stays mounted, so it comes back as it was. */}
              <div className={`flex min-h-0 min-w-0 flex-col ${panelExpanded ? 'w-0 flex-none overflow-hidden' : 'flex-1'}`} {...(panelExpanded ? { inert: '' } : {})}>
                <div className="relative min-h-0 flex-1">
                  <div className={statsOpen || digestOpen ? 'hidden' : 'h-full'}>{selectedId ? <Viewport /> : <Home />}</div>
                  {statsOpen && <Stats />}
                  {digestOpen && <DigestView />}
                  {overlay && <PanelOverlay panel={overlay} />}
                </div>
                <TerminalDrawer />
              </div>
              {docked.right && rightOpen && !panelExpanded && <Divider panel="right" />}
              {docked.right && rightOpen && <RightPanel />}
            </main>
          </div>
        </div>
        <ShortcutsSheet />
        <SettingsPanel />
        <PatchNotes />
        <Toasts />
      </div>
      <UsageModal />
      <ConfirmHost />
      {!network.online && <OfflineSplash retrying={network.retrying} onRetry={() => void network.retry()} />}
    </>
  )
}
