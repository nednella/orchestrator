import { useOrchestrator } from '../OrchestratorContext'
import { ACCESSES, ACCESS_HINT, ACCESS_NAME, newThreadAccess } from '../access'
import { PLACE_LABEL, placeFor } from '../place'
import { SETTING_DEFAULTS } from '../settingsDefaults'
import type { Access, Place, PromptSend } from '../types'
import { AccessIcon } from './AccessIcon'
import { SelectMenu } from './SelectMenu'
import type { SelectOption } from './SelectMenu'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'
import { TextField } from './TextField'

const PLACES: SelectOption<Place>[] = [
  { value: 'worktree', label: PLACE_LABEL.worktree },
  { value: 'project', label: PLACE_LABEL.project },
]

const ACCESS_OPTIONS: SelectOption<Access>[] = ACCESSES.map((value) => ({ value, label: ACCESS_NAME[value], icon: <AccessIcon access={value} />, description: `${ACCESS_HINT[value]}.` }))

const PROMPT_SEND: SelectOption<PromptSend>[] = [
  { value: 'auto', label: 'Auto' },
  { value: 'manual', label: 'Manual' },
]

export function ThreadsSettings() {
  const { project, settings, setProjectsDir, setThreadPlace, setThreadAccess, setPromptSend, report } = useOrchestrator('project', 'settings', 'setProjectsDir', 'setThreadPlace', 'setThreadAccess', 'setPromptSend', 'report')
  const place = placeFor(settings.threadPlace, false)
  const access = newThreadAccess(settings.threadAccess)
  return (
    <SettingsPage>
      <SettingsSection title="New projects">
        <SettingsRow
          title="Add project starts in"
          description='Where Add project makes a new project and its folder browser starts. Leave empty to use "~/code".'
          control={<TextField label="Add project base directory" value={settings.projectsDir} placeholder="~/code" onCommit={(dir) => report(() => setProjectsDir(dir))} />}
          onReset={settings.projectsDir !== SETTING_DEFAULTS.projectsDir ? () => report(() => setProjectsDir(SETTING_DEFAULTS.projectsDir)) : undefined}
        />
      </SettingsSection>
      <SettingsSection title="New threads">
        <SettingsRow
          title="Workspace"
          description="Where new threads start."
          control={<SelectMenu label="Default workspace" options={PLACES} value={place} onChange={(value) => report(() => setThreadPlace(value))} />}
          onReset={place !== SETTING_DEFAULTS.threadPlace ? () => report(() => setThreadPlace(SETTING_DEFAULTS.threadPlace)) : undefined}
        />
        <SettingsRow
          title="Permissions"
          description="Default permissions for new threads. A thread can change them in the model menu."
          control={<SelectMenu label="Default permissions" options={ACCESS_OPTIONS} value={access} onChange={(value) => report(() => setThreadAccess(value))} />}
          onReset={access !== SETTING_DEFAULTS.threadAccess ? () => report(() => setThreadAccess(SETTING_DEFAULTS.threadAccess)) : undefined}
        />
        {project && (
          <SettingsRow
            title="Send the first prompt"
            description={`Auto sends what a new task starts with at once. Manual leaves it as a draft in the chat. Applies to ${project.name}.`}
            control={<SelectMenu label="Send the first prompt" options={PROMPT_SEND} value={settings.promptSend} onChange={(value) => report(() => setPromptSend(value))} />}
          />
        )}
      </SettingsSection>
    </SettingsPage>
  )
}
