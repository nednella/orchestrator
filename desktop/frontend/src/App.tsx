import { useState } from 'react'
import type { CSSProperties } from 'react'
import { ConfirmHost } from './components/ConfirmHost'
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
import { ShellStrip } from './components/ShellStrip'
import { Splash, type SplashStage } from './components/Splash'
import { Stats } from './components/Stats'
import { Toasts } from './components/Toasts'
import { TopBar } from './components/TopBar'
import { Viewport } from './components/Viewport'
import { useAgentos } from './AgentosContext'
import { useConfirming } from './confirm'
import { useLayout } from './LayoutContext'
import { useFocusedPanel } from './useFocusedPanel'
import { useFullscreen } from './useFullscreen'
import { useDigestToasts } from './useDigestToasts'
import { useNetwork } from './useNetwork'
import { useShortcuts } from './useShortcuts'
import { useUiCommands } from './useUiCommands'

const appEnterClass: Record<SplashStage, string> = {
  playing: 'app-enter',
  leaving: 'app-enter shown',
  done: '',
}

export function App() {
  const network = useNetwork()
  const confirming = useConfirming()
  useShortcuts(network.online && !confirming)
  useUiCommands()
  useDigestToasts()
  useFocusedPanel()
  const fullscreen = useFullscreen()
  const { selectedId } = useAgentos('selectedId')
  const { mode, docked, sidebarOpen, rightOpen, overlay, statsOpen, digestOpen } = useLayout()
  const [splash, setSplash] = useState<SplashStage>(() =>
    matchMedia('(prefers-reduced-motion: reduce)').matches ? 'done' : 'playing',
  )

  return (
    <>
      <div
        className={`flex h-full flex-col bg-app ${appEnterClass[splash]}`}
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
              <div className="flex min-h-0 min-w-0 flex-1 flex-col">
                <div className="relative min-h-0 flex-1">
                  <div className={statsOpen || digestOpen ? 'hidden' : 'h-full'}>{selectedId ? <Viewport /> : <Home />}</div>
                  {statsOpen && <Stats />}
                  {digestOpen && <DigestView />}
                  {overlay && <PanelOverlay panel={overlay} />}
                </div>
                <ShellStrip />
              </div>
              {docked.right && rightOpen && <Divider panel="right" />}
              {docked.right && rightOpen && <RightPanel />}
            </main>
          </div>
        </div>
        <ShortcutsSheet />
        <SettingsPanel />
        <PatchNotes />
        <Toasts />
      </div>
      <ConfirmHost />
      {!network.online && <OfflineSplash retrying={network.retrying} onRetry={() => void network.retry()} />}
      {splash !== 'done' && <Splash stage={splash} onStage={setSplash} />}
    </>
  )
}
