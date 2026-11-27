import { useOrchestrator } from '../OrchestratorContext'
import type { DigestSchedule } from '../types'
import { DataDirRow } from './DataDirRow'
import { SelectMenu } from './SelectMenu'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'
import { Switch } from './Switch'
import { TextField } from './TextField'

const SCHEDULES: { value: DigestSchedule; label: string }[] = [
  { value: 'weekly', label: 'Weekly' },
  { value: 'off', label: 'Off' },
]

export function GeneralSettings() {
  const { project, settings, setKeepAwake, setDigestSchedule, setProjectsDir, report } = useOrchestrator('project', 'settings', 'setKeepAwake', 'setDigestSchedule', 'setProjectsDir', 'report')
  return (
    <SettingsPage>
      <SettingsSection title="Storage">
        <DataDirRow dir={settings.dataDir} fixed={settings.dataDirFixed} />
        <SettingsRow
          title="Projects folder"
          description="Where Add project makes a new project, and where its folder browser starts. Empty uses ~/code."
          control={<TextField label="Projects folder" value={settings.projectsDir} placeholder="~/code" onCommit={(dir) => report(() => setProjectsDir(dir))} />}
        />
      </SettingsSection>
      <SettingsSection title="System">
        <SettingsRow
          title="Keep Mac awake"
          description="Stops the Mac idle-sleeping while a session works. A project that sets its own in the config file keeps it."
          control={<Switch label="Keep Mac awake" checked={settings.keepAwake} onChange={(on) => report(() => setKeepAwake(on))} />}
        />
      </SettingsSection>
      {project && (
        <SettingsSection title={project.name}>
          <SettingsRow
            title="News schedule"
            description="Weekly checks for news by itself. Off leaves it to you."
            control={<SelectMenu label="News schedule" options={SCHEDULES} value={settings.digestSchedule} onChange={(value) => report(() => setDigestSchedule(value))} />}
          />
        </SettingsSection>
      )}
    </SettingsPage>
  )
}
