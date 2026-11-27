import { useOrchestrator } from '../OrchestratorContext'
import { DataDirRow } from './DataDirRow'
import { SettingsPage } from './SettingsPage'
import { SettingsSection } from './SettingsSection'

export function StorageSettings() {
  const { settings } = useOrchestrator('settings')
  return (
    <SettingsPage>
      <SettingsSection title="Data folder" plain>
        <div className="rounded-xl border border-(--border-rule) bg-(--bg-card)">
          <DataDirRow dir={settings.dataDir} fixed={settings.dataDirFixed} />
        </div>
      </SettingsSection>
    </SettingsPage>
  )
}
