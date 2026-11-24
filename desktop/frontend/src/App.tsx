import { useState } from 'react'
import type { CSSProperties } from 'react'
import { DigestView } from './components/DigestView'
import { Divider } from './components/Divider'
import { Home } from './components/Home'
import { IssueDialog } from './components/IssueDialog'
import { MobileTabs } from './components/MobileTabs'
import { OfflineSplash } from './components/OfflineSplash'
import { RightPanel } from './components/RightPanel'
import { Palette } from './components/Palette'
import { ProjectRail } from './components/ProjectRail'
import { PatchNotes } from './components/PatchNotes'
import { SettingsPanel } from './components/SettingsPanel'
import { ShortcutsSheet } from './components/ShortcutsSheet'
import { ShellPanel } from './components/ShellPanel'
import { ShellStrip } from './components/ShellStrip'
import { ThreadSidebar } from './components/ThreadSidebar'
import { Splash, type SplashStage } from './components/Splash'
import { Stats } from './components/Stats'
import { Toasts } from './components/Toasts'
import { TopBar } from './components/TopBar'
import { Viewport } from './components/Viewport'
import { useAgentos } from './AgentosContext'
import { useLayout } from './LayoutContext'
import { useFocusedPanel } from './useFocusedPanel'
import { useDigestToasts } from './useDigestToasts'
import { useNetwork } from './useNetwork'
import { useShortcuts } from './useShortcuts'
import { useUiCommands } from './useUiCommands'

const drag = { '--wails-draggable': 'drag' } as CSSProperties

const appEnterClass: Record<SplashStage, string> = {
  playing: 'app-enter',
  leaving: 'app-enter shown',
  done: '',
}

export function App() {
  const network = useNetwork()
  useShortcuts(network.online)
  useUiCommands()
  useDigestToasts()
  useFocusedPanel()
  const { selectedId } = useAgentos('selectedId')
  const { mode, sidebarOpen, rightOpen, mobilePanel, statsOpen, digestOpen } = useLayout()
  const narrow = mode === 'narrow'
  const showCentre = !narrow || mobilePanel === 'session' || mobilePanel === 'queue' || mobilePanel === 'notes'
  const home = narrow ? mobilePanel === 'queue' || mobilePanel === 'notes' || (mobilePanel === 'session' && !selectedId) : !selectedId
  const [splash, setSplash] = useState<SplashStage>(() =>
    matchMedia('(prefers-reduced-motion: reduce)').matches ? 'done' : 'playing',
  )

  return (
    <>
      <div
        className={`flex h-full flex-col bg-app ${appEnterClass[splash]}`}
        data-mode={mode}
        {...(network.online ? {} : { inert: '' })}
      >
        <div className="relative flex min-h-0 flex-1">
          <div className={`flex min-h-0 flex-col ${narrow && sidebarOpen ? 'min-w-0 flex-1' : 'flex-none'}`}>
            <div className="flex-none" style={{ ...drag, height: 'var(--topbar-h)' }} />
            <div className="flex min-h-0 flex-1">
              <ProjectRail />
              {sidebarOpen && <ThreadSidebar />}
            </div>
          </div>
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
              {narrow && mobilePanel === 'details' && <RightPanel />}
              {narrow && mobilePanel === 'shell' && <ShellPanel />}
            </main>
          </div>
        </div>
        {narrow && <MobileTabs />}
        <Palette />
        <IssueDialog />
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
