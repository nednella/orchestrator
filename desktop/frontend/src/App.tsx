import { useState } from 'react'
import type { CSSProperties } from 'react'
import { DigestView } from './components/DigestView'
import { Divider } from './components/Divider'
import { Home } from './components/Home'
import { LeftSidebar } from './components/LeftSidebar'
import { MobileTabs } from './components/MobileTabs'
import { OfflineSplash } from './components/OfflineSplash'
import { RightPanel } from './components/RightPanel'
import { PatchNotes } from './components/PatchNotes'
import { SettingsPanel } from './components/SettingsPanel'
import { ShortcutsSheet } from './components/ShortcutsSheet'
import { ShellPanel } from './components/ShellPanel'
import { ShellStrip } from './components/ShellStrip'
import { SidebarToggle } from './components/SidebarToggle'
import { Splash, type SplashStage } from './components/Splash'
import { Stats } from './components/Stats'
import { Toasts } from './components/Toasts'
import { TopBar } from './components/TopBar'
import { Viewport } from './components/Viewport'
import { useAgentos } from './AgentosContext'
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

// Where the sidebar toggle sits, and where the title bar's own content starts after it.
const titleInsets = (toggleLeft: string) => ({ '--sidebar-toggle-left': toggleLeft, '--title-start': `calc(${toggleLeft} + 40px)` }) as CSSProperties

export function App() {
  const network = useNetwork()
  useShortcuts(network.online)
  useUiCommands()
  useDigestToasts()
  useFocusedPanel()
  const fullscreen = useFullscreen()
  const { selectedId } = useAgentos('selectedId')
  const { mode, sidebarOpen, rightOpen, mobilePanel, statsOpen, digestOpen } = useLayout()
  const narrow = mode === 'narrow'
  const showCentre = !narrow || mobilePanel === 'session'
  const home = !selectedId && showCentre
  const [splash, setSplash] = useState<SplashStage>(() =>
    matchMedia('(prefers-reduced-motion: reduce)').matches ? 'done' : 'playing',
  )

  return (
    <>
      <div
        className={`flex h-full flex-col bg-app ${appEnterClass[splash]}`}
        style={titleInsets(fullscreen ? 'var(--corner-pad)' : 'var(--traffic-light-zone)')}
        data-mode={mode}
        {...(network.online ? {} : { inert: '' })}
      >
        <div className="relative flex min-h-0 flex-1">
          {sidebarOpen && <LeftSidebar />}
          {!narrow && sidebarOpen && <Divider panel="sidebar" />}
          <div className={`flex min-w-0 flex-1 flex-col ${narrow && sidebarOpen ? 'hidden' : ''}`}>
            <TopBar />
            <main className="relative flex min-h-0 min-w-0 flex-1">
              <div className={`flex min-h-0 min-w-0 flex-1 flex-col ${showCentre ? '' : 'hidden'}`}>
                <div className="min-h-0 flex-1">
                  <div className={statsOpen || digestOpen ? 'hidden' : 'h-full'}>
                    {selectedId && (
                      <div className={home ? 'hidden' : 'h-full'}>
                        <Viewport />
                      </div>
                    )}
                    {home && <Home />}
                  </div>
                  {statsOpen && <Stats />}
                  {digestOpen && <DigestView />}
                </div>
                {!narrow && <ShellStrip />}
              </div>
              {rightOpen && <Divider panel="right" />}
              {rightOpen && <RightPanel />}
              {narrow && mobilePanel === 'panel' && <RightPanel />}
              {narrow && mobilePanel === 'shell' && <ShellPanel />}
            </main>
          </div>
          <SidebarToggle />
        </div>
        {narrow && <MobileTabs />}
        <ShortcutsSheet />
        <SettingsPanel />
        <PatchNotes />
        <Toasts />
      </div>
      {!network.online && <OfflineSplash retrying={network.retrying} onRetry={() => void network.retry()} />}
      {splash !== 'done' && <Splash stage={splash} onStage={setSplash} />}
    </>
  )
}
