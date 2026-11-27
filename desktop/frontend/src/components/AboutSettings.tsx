import { useOrchestrator } from '../OrchestratorContext'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'

export function AboutSettings() {
  const { setOverlay } = useOrchestrator('setOverlay')
  return (
    <SettingsPage>
      <SettingsSection title="App">
        <SettingsRow
          title="What's new"
          description="The changes in the version you run."
          control={
            <button className="btn h-7 flex-none px-2.5" onClick={() => setOverlay('changelog')}>
              Open
            </button>
          }
        />
      </SettingsSection>
    </SettingsPage>
  )
}
