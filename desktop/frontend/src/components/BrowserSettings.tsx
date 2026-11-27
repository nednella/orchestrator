import { useOrchestrator } from '../OrchestratorContext'
import { SETTING_DEFAULTS } from '../settingsDefaults'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'
import { Switch } from './Switch'

export function BrowserSettings() {
  const { settings, setBrowserEnabled, report } = useOrchestrator('settings', 'setBrowserEnabled', 'report')
  return (
    <SettingsPage perProject>
      <SettingsSection title="Agent access">
        <SettingsRow
          title="Agent browser access"
          description="Allow agents in this project to use the browser and the media commands. Applies when a task next starts."
          control={<Switch label="Agent browser access" checked={settings.browserEnabled} onChange={(on) => report(() => setBrowserEnabled(on))} />}
          onReset={settings.browserEnabled !== SETTING_DEFAULTS.browserEnabled ? () => report(() => setBrowserEnabled(SETTING_DEFAULTS.browserEnabled)) : undefined}
        />
      </SettingsSection>
    </SettingsPage>
  )
}
