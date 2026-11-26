import { useOrchestrator } from '../OrchestratorContext'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'
import { Switch } from './Switch'

export function BrowserSettings() {
  const { settings, setBrowserEnabled, report } = useOrchestrator('settings', 'setBrowserEnabled', 'report')
  return (
    <SettingsPage perProject>
      <SettingsSection title="Browser">
        <SettingsRow
          title="Browser enabled"
          description="Tells new tasks about the browser and media commands."
          control={<Switch label="Browser enabled" checked={settings.browserEnabled} onChange={(on) => report(() => setBrowserEnabled(on))} />}
        />
      </SettingsSection>
    </SettingsPage>
  )
}
