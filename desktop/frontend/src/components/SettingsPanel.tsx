import { X } from 'lucide-react'
import { useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { sectionLabel } from '../settingsSections'
import type { SettingsSectionId } from '../settingsSections'
import { AboutSettings } from './AboutSettings'
import { AgentSettings } from './AgentSettings'
import { AppearanceSettings } from './AppearanceSettings'
import { BrowserSettings } from './BrowserSettings'
import { GeneralSettings } from './GeneralSettings'
import { GitSettings } from './GitSettings'
import { Overlay } from './Overlay'
import { SettingsNav } from './SettingsNav'

const PAGES: Record<SettingsSectionId, () => JSX.Element> = {
  general: GeneralSettings,
  appearance: AppearanceSettings,
  agents: AgentSettings,
  browser: BrowserSettings,
  git: GitSettings,
  about: AboutSettings,
}

export function SettingsPanel() {
  const { overlay, setOverlay } = useOrchestrator('overlay', 'setOverlay')
  const [section, setSection] = useState<SettingsSectionId>('general')
  if (overlay !== 'settings') return null
  const Page = PAGES[section]

  return (
    <Overlay label="Settings" size="settings" onClose={() => setOverlay(null)}>
      <div className="flex min-h-0 flex-1">
        <SettingsNav section={section} onSelect={setSection} />
        <div className="flex min-w-0 flex-1 flex-col">
          <header className="flex h-12 flex-none items-center justify-between border-b border-line ps-5 pe-2">
            <p className="text-body">
              <span className="text-dim">Settings</span>
              <span className="px-1.5 text-dim">/</span>
              <span className="font-medium">{sectionLabel(section)}</span>
            </p>
            <button className="btn btn-ghost size-7 justify-center px-0" aria-label="Close" title="Close (Esc)" onClick={() => setOverlay(null)}>
              <X size={16} aria-hidden="true" />
            </button>
          </header>
          <div className="min-h-0 flex-1 overflow-y-auto">
            <div className="mx-auto w-full max-w-4xl px-6 pt-6 pb-12">
              <Page />
            </div>
          </div>
        </div>
      </div>
    </Overlay>
  )
}
