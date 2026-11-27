import { useOrchestrator } from '../OrchestratorContext'
import { themeHalves } from '../themes'
import type { Appearance } from '../themes'
import { AccentChoices } from './AccentChoices'
import { SettingsRow } from './SettingsRow'

const ROWS: { mode: Appearance; title: string; description: string }[] = [
  { mode: 'dark', title: 'Dark', description: 'Buttons, links and focus rings in dark mode.' },
  { mode: 'light', title: 'Light', description: 'Buttons, links and focus rings in light mode.' },
]

// AccentSettings picks the accent of each appearance: the theme's own, a preset or any color.
export function AccentSettings() {
  const { settings, setAccents, report } = useOrchestrator('settings', 'setAccents', 'report')
  const halves = themeHalves(settings)
  const accents = { dark: settings.darkAccent, light: settings.lightAccent }
  return (
    <div className="rounded-xl border border-(--border-rule) bg-(--bg-card)">
      {ROWS.map(({ mode, title, description }) => (
        <SettingsRow
          key={mode}
          title={title}
          description={description}
          control={
            <AccentChoices
              mode={mode}
              theme={halves[mode]}
              accent={accents[mode]}
              onPick={(accent) => {
                const next = { ...accents, [mode]: accent }
                report(() => setAccents(next.dark, next.light))
              }}
            />
          }
        />
      ))}
    </div>
  )
}
