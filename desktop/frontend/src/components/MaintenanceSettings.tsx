import { useOrchestrator } from '../OrchestratorContext'
import { SETTING_DEFAULTS } from '../settingsDefaults'
import type { DigestSchedule } from '../types'
import { DataDirRow } from './DataDirRow'
import { SelectMenu } from './SelectMenu'
import type { SelectOption } from './SelectMenu'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'
import { Switch } from './Switch'

const SCHEDULES: SelectOption<DigestSchedule>[] = [
  { value: 'weekly', label: 'Weekly' },
  { value: 'off', label: 'Off' },
]

export function MaintenanceSettings() {
  const { project, settings, setKeepAwake, setDigestSchedule, report } = useOrchestrator('project', 'settings', 'setKeepAwake', 'setDigestSchedule', 'report')
  return (
    <SettingsPage>
      <SettingsSection title="Background activity">
        <SettingsRow
          title="Keep awake while agents work"
          description="Stop the Mac from going to sleep while agents are working, and let it sleep once they finish. Closing a laptop lid still puts it to sleep. A project that sets its own in the config file keeps it."
          control={<Switch label="Keep awake while agents work" checked={settings.keepAwake} onChange={(on) => report(() => setKeepAwake(on))} />}
          onReset={settings.keepAwake !== SETTING_DEFAULTS.keepAwake ? () => report(() => setKeepAwake(SETTING_DEFAULTS.keepAwake)) : undefined}
        />
        {project && (
          <SettingsRow
            title="News schedule"
            description={`Weekly checks for news by itself. Off leaves it to you. Applies to ${project.name}.`}
            control={<SelectMenu label="News schedule" options={SCHEDULES} value={settings.digestSchedule} onChange={(value) => report(() => setDigestSchedule(value))} />}
          />
        )}
      </SettingsSection>
      <SettingsSection title="Storage">
        <DataDirRow dir={settings.dataDir} fixed={settings.dataDirFixed} />
      </SettingsSection>
    </SettingsPage>
  )
}
