import { useState } from 'react'
import { DigestView } from './components/DigestView'
import { NewsIndex } from './components/NewsIndex'
import { NewsView } from './components/NewsView'
import { Divider } from './components/Divider'
import { IssueDialog } from './components/IssueDialog'
import { MobileTabs } from './components/MobileTabs'
import { OfflineSplash } from './components/OfflineSplash'
import { Palette } from './components/Palette'
import { PatchNotes } from './components/PatchNotes'
import { QueueNotes } from './components/QueueNotes'
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
import { useNetwork } from './useNetwork'
import { useNewsToasts } from './useNewsToasts'
import { useShortcuts } from './useShortcuts'
import { useUiCommands } from './useUiCommands'

const appEnterClass: Record<SplashStage, string> = {
  playing: 'app-enter',
  leaving: 'app-enter shown',
  done: '',
}

export function App() {
  const network = useNetwork()
  useShortcuts(network.online)
  useUiCommands()
  useNewsToasts()
  useFocusedPanel()
  const { selectedId } = useAgentos('selectedId')
  const { mode, sidebarOpen, mobilePanel, statsOpen, newsOpen, newsPage } = useLayout()
  const narrow = mode === 'narrow'
  const showCentre = !narrow || mobilePanel === 'session' || mobilePanel === 'queue' || mobilePanel === 'notes'
  const home = narrow ? mobilePanel === 'queue' || mobilePanel === 'notes' : !selectedId
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
        <TopBar />
        <main className={`relative flex min-h-0 min-w-0 flex-1 ${narrow ? 'p-2' : 'p-3'}`}>
          {sidebarOpen && <ThreadSidebar />}
          {!narrow && sidebarOpen && <Divider panel="sidebar" />}
          <div className={`min-h-0 min-w-0 flex-1 ${showCentre ? '' : 'hidden'}`}>
            <div className={statsOpen || newsOpen ? 'hidden' : 'h-full'}>
              {(narrow || selectedId) && (
                <div className={home ? 'hidden' : 'h-full'}>
                  <Viewport />
                </div>
              )}
              {home && <QueueNotes />}
            </div>
            {statsOpen && <Stats />}
            {newsOpen && newsPage === 'index' && <NewsIndex />}
            {newsOpen && newsPage === 'tldr' && <NewsView />}
            {newsOpen && newsPage === 'digest' && <DigestView />}
          </div>
          {narrow && mobilePanel === 'shell' && <ShellPanel />}
        </main>
        {narrow && <MobileTabs />}
        {!narrow && <ShellStrip />}
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
