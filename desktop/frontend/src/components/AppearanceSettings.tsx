import { useOrchestrator } from '../OrchestratorContext'
import { useLayout } from '../LayoutContext'
import type { DarkTheme, ThemeSetting } from '../types'
import { SelectMenu } from './SelectMenu'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'

const THEMES: { value: ThemeSetting; label: string }[] = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
]

const DARK_THEMES: { value: DarkTheme; label: string }[] = [
  { value: 'black', label: 'Black' },
  { value: 'violet', label: 'Violet' },
]

export function AppearanceSettings() {
  const { settings, setTheme, setThemes, report } = useOrchestrator('settings', 'setTheme', 'setThemes', 'report')
  const { scale, zoom } = useLayout()
  return (
    <SettingsPage>
      <SettingsSection title="Colors">
        <SettingsRow
          title="Theme"
          description="System follows the macOS appearance."
          control={<SelectMenu label="Theme" options={THEMES} value={settings.theme} onChange={(value) => report(() => setTheme(value))} />}
        />
        <SettingsRow
          title="Dark theme"
          description="Shown when the theme is dark, or System follows a dark macOS."
          control={<SelectMenu label="Dark theme" options={DARK_THEMES} value={settings.darkTheme} onChange={(value) => report(() => setThemes(value, settings.lightTheme))} />}
        />
      </SettingsSection>
      <SettingsSection title="Interface">
        <SettingsRow
          title="Text size"
          description="Scales the interface and the terminal."
          control={
            <div role="group" aria-label="Text size" className="flex flex-none items-center gap-1">
              <button className="btn btn-ghost h-7 w-7 justify-center px-0" aria-label="Smaller text" onClick={() => zoom(-1)}>
                −
              </button>
              <button className="btn btn-ghost mono h-7 w-14 justify-center px-0" title="Reset text size" aria-label="Reset text size" onClick={() => zoom(0)}>
                {Math.round(scale * 100)}%
              </button>
              <button className="btn btn-ghost h-7 w-7 justify-center px-0" aria-label="Larger text" onClick={() => zoom(1)}>
                +
              </button>
            </div>
          }
        />
      </SettingsSection>
    </SettingsPage>
  )
}
