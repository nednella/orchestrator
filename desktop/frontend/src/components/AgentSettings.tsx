import { useOrchestrator } from '../OrchestratorContext'
import { ACCESSES, ACCESS_NAME, newThreadAccess } from '../access'
import { PLACE_LABEL, placeFor } from '../place'
import type { Access, Place, PromptSend } from '../types'
import { SelectField } from './SelectField'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'

const PLACES: { value: Place; label: string }[] = [
  { value: 'worktree', label: PLACE_LABEL.worktree },
  { value: 'project', label: PLACE_LABEL.project },
]

const ACCESS_OPTIONS: { value: Access; label: string }[] = ACCESSES.map((value) => ({ value, label: ACCESS_NAME[value] }))

const PROMPT_SEND: { value: PromptSend; label: string }[] = [
  { value: 'auto', label: 'Auto' },
  { value: 'manual', label: 'Manual' },
]

export function AgentSettings() {
  const { project, settings, setThreadPlace, setThreadAccess, setPromptSend, report } = useOrchestrator('project', 'settings', 'setThreadPlace', 'setThreadAccess', 'setPromptSend', 'report')
  return (
    <SettingsPage>
      <SettingsSection title="New threads">
        <SettingsRow
          title="New threads start in"
          description="Where a new thread works."
          control={<SelectField label="New threads start in" options={PLACES} value={placeFor(settings.threadPlace, false)} onChange={(value) => report(() => setThreadPlace(value))} />}
        />
        <SettingsRow
          title="Default access"
          description="A thread can change it in the model menu."
          control={<SelectField label="Default access" options={ACCESS_OPTIONS} value={newThreadAccess(settings.threadAccess)} onChange={(value) => report(() => setThreadAccess(value))} />}
        />
      </SettingsSection>
      {project && (
        <SettingsSection title={project.name}>
          <SettingsRow
            title="Send the first prompt"
            description="Auto sends what a new task starts with at once. Manual leaves it as a draft in the chat."
            control={<SelectField label="Send the first prompt" options={PROMPT_SEND} value={settings.promptSend} onChange={(value) => report(() => setPromptSend(value))} />}
          />
        </SettingsSection>
      )}
    </SettingsPage>
  )
}
