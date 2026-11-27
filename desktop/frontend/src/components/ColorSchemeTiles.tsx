import { useOrchestrator } from '../OrchestratorContext'
import type { ThemeSetting } from '../types'
import { ThemeWireframe } from './ThemeWireframe'

const TILES: { value: ThemeSetting; label: string; aria: string }[] = [
  { value: 'system', label: 'System', aria: 'Follow the system appearance' },
  { value: 'light', label: 'Light', aria: 'Use light mode' },
  { value: 'dark', label: 'Dark', aria: 'Use dark mode' },
]

export function ColorSchemeTiles() {
  const { settings, setTheme, report } = useOrchestrator('settings', 'setTheme', 'report')
  const panes = {
    system: [
      { theme: settings.lightTheme, clip: 'left' as const },
      { theme: settings.darkTheme, clip: 'right' as const },
    ],
    light: [{ theme: settings.lightTheme }],
    dark: [{ theme: settings.darkTheme }],
  }
  return (
    <div role="group" aria-label="Appearance mode" className="grid w-full grid-cols-3 gap-3">
      {TILES.map(({ value, label, aria }) => {
        const picked = settings.theme === value
        return (
          <button
            key={value}
            type="button"
            aria-label={aria}
            aria-pressed={picked}
            className={`flex cursor-pointer flex-col items-stretch gap-1.5 rounded-xl border p-2 transition-colors focus-visible:-outline-offset-7 ${picked ? 'border-transparent bg-(--bg-row-active) ring-2 ring-(--accent) ring-inset' : 'border-(--border-rule) bg-(--bg-card) hover:bg-(--bg-row-hover)'}`}
            onClick={() => report(() => setTheme(value))}
          >
            <ThemeWireframe panes={panes[value]} />
            <span className={`flex items-center justify-center text-small font-medium ${picked ? 'text-ink' : 'text-soft'}`}>{label}</span>
          </button>
        )
      })}
    </div>
  )
}
