import { useOrchestrator } from '../OrchestratorContext'
import type { CleanupMode } from '../types'
import { SelectMenu } from './SelectMenu'
import type { SelectOption } from './SelectMenu'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'

const MODES: SelectOption<CleanupMode>[] = [
  { value: 'auto', label: 'Auto' },
  { value: 'manual', label: 'Manual' },
]

export function WorktreesSettings() {
  const { settings, setCleanup, report } = useOrchestrator('settings', 'setCleanup', 'report')
  return (
    <SettingsPage perProject>
      <SettingsSection title="Cleanup">
        <SettingsRow
          title="Clean up after a merge"
          description="Remove worktrees whose pull request is merged and whose commits are included in the default branch. Auto removes them at once and Manual asks first. The branch stays; unsaved or unpushed work always blocks."
          control={<SelectMenu label="Clean up after a merge" options={MODES} value={settings.cleanup.merge} onChange={(value) => report(() => setCleanup('merge', value))} />}
        />
        <SettingsRow
          title="Clean up after a close"
          description="Auto cleans up when you close the pull request. Manual asks first."
          control={<SelectMenu label="Clean up after a close" options={MODES} value={settings.cleanup.close} onChange={(value) => report(() => setCleanup('close', value))} />}
        />
      </SettingsSection>
    </SettingsPage>
  )
}
