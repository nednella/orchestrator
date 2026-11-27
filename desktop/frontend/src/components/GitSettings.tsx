import { useOrchestrator } from '../OrchestratorContext'
import type { CleanupMode } from '../types'
import { SelectMenu } from './SelectMenu'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'

const MODES: { value: CleanupMode; label: string }[] = [
  { value: 'auto', label: 'Auto' },
  { value: 'manual', label: 'Manual' },
]

export function GitSettings() {
  const { settings, setCleanup, report } = useOrchestrator('settings', 'setCleanup', 'report')
  return (
    <SettingsPage perProject>
      <SettingsSection title="Clean up">
        <SettingsRow
          title="Clean up after a merge"
          description="Auto removes the worktree orchestrator made; the branch stays. Manual asks first. Unsaved or unpushed work always blocks."
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
