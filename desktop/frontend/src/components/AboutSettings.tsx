import { useOrchestrator } from '../OrchestratorContext'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'

export function AboutSettings() {
  const { version, setOverlay } = useOrchestrator('version', 'setOverlay')
  return (
    <SettingsPage>
      <SettingsSection title="App">
        <SettingsRow
          title="Version"
          description={version ? `orchestrator ${version}` : 'A development build'}
          control={
            <button className="btn h-7 flex-none px-2.5" onClick={() => setOverlay('changelog')}>
              Patch notes
            </button>
          }
        />
      </SettingsSection>
    </SettingsPage>
  )
}
