import { useLayout } from '../LayoutContext'
import { AccentSettings } from './AccentSettings'
import { ColorSchemeTiles } from './ColorSchemeTiles'
import { SettingsPage } from './SettingsPage'
import { SettingsRow } from './SettingsRow'
import { SettingsSection } from './SettingsSection'
import { ThemeCards } from './ThemeCards'

const heading = 'flex min-h-7 items-center px-4 text-title font-semibold'

export function AppearanceSettings() {
  const { scale, zoom } = useLayout()
  return (
    <SettingsPage>
      <SettingsSection title="Colors & themes" plain>
        <div className="space-y-3">
          <h3 className={heading}>Color scheme</h3>
          <ColorSchemeTiles />
          <h3 className={`${heading} min-h-8 pt-5`}>Themes</h3>
          <ThemeCards />
          <h3 className={`${heading} min-h-8 pt-5`}>Accent</h3>
          <AccentSettings />
        </div>
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
              <button className="btn btn-ghost tabular-nums h-7 w-14 justify-center px-0" title="Reset text size" aria-label="Reset text size" onClick={() => zoom(0)}>
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
