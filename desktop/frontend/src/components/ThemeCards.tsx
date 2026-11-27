import { useOrchestrator } from '../OrchestratorContext'
import type { DarkTheme } from '../types'
import { ThemeCard } from './ThemeCard'

const DARK_THEMES: { value: DarkTheme; label: string }[] = [
  { value: 'black', label: 'Black' },
  { value: 'violet', label: 'Violet' },
]

export function ThemeCards() {
  const { settings, setThemes, report } = useOrchestrator('settings', 'setThemes', 'report')
  return (
    <div className="grid w-full grid-cols-[repeat(auto-fit,minmax(min(100%,16rem),1fr))] gap-2">
      {DARK_THEMES.map(({ value, label }) => (
        <ThemeCard key={value} theme={value} label={label} active={settings.darkTheme === value} onUse={() => report(() => setThemes(value, settings.lightTheme))} />
      ))}
    </div>
  )
}
