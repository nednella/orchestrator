import { ACCENT_PRESETS, accentChoice, presetToken } from '../accent'
import type { AccentPreset } from '../accent'
import type { Appearance } from '../themes'
import type { ThemeName } from '../types'
import { AccentCustomSwatch } from './AccentCustomSwatch'
import { AccentSwatch } from './AccentSwatch'

type AccentChoicesProps = { mode: Appearance; theme: ThemeName; accent: string; onPick(accent: string): void }

// AccentChoices is the accents one appearance can show: its theme's own, the presets, and a custom color, all discs of one size in one run.
export function AccentChoices({ mode, theme, accent, onPick }: AccentChoicesProps) {
  const color = (preset: AccentPreset) => getComputedStyle(document.documentElement).getPropertyValue(presetToken(mode, preset)).trim()
  const picked = accentChoice(accent, color)
  return (
    <div role="group" aria-label={`Accent for ${mode} mode`} className="flex flex-wrap items-center gap-1">
      <AccentSwatch label="Theme" mode={mode} preview={theme} fill="var(--accent)" picked={picked === 'theme'} onPick={() => onPick('')} />
      {ACCENT_PRESETS.map(({ id, label }) => (
        <AccentSwatch key={id} label={label} mode={mode} fill={`var(${presetToken(mode, id)})`} picked={picked === id} onPick={() => onPick(color(id))} />
      ))}
      <AccentCustomSwatch mode={mode} theme={theme} saved={accent} custom={picked === 'custom' ? accent : ''} start={color('blue')} onPick={onPick} />
    </div>
  )
}
