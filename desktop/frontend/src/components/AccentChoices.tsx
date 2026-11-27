import { ACCENT_PRESETS, accentChoice, presetToken } from '../accent'
import type { AccentPreset } from '../accent'
import type { Appearance } from '../themes'
import type { ThemeName } from '../types'
import { AccentCustomSwatch } from './AccentCustomSwatch'
import { AccentSwatch } from './AccentSwatch'

type AccentChoicesProps = { mode: Appearance; theme: ThemeName; accent: string; onPick(accent: string): void }

const divider = <span aria-hidden="true" className="mx-1 h-5 w-px flex-none bg-(--border-rule)" />

// AccentChoices is the accents one appearance can show: its theme's own, the presets, and a custom color.
export function AccentChoices({ mode, theme, accent, onPick }: AccentChoicesProps) {
  const color = (preset: AccentPreset) => getComputedStyle(document.documentElement).getPropertyValue(presetToken(mode, preset)).trim()
  const picked = accentChoice(accent, color)
  return (
    <div role="group" aria-label={`Accent for ${mode} mode`} className="flex flex-wrap items-center gap-0.5">
      <AccentSwatch label="Theme" mode={mode} preview={theme} fill="var(--accent)" picked={picked === 'theme'} onPick={() => onPick('')} />
      {divider}
      {ACCENT_PRESETS.map(({ id, label }) => (
        <AccentSwatch key={id} label={label} mode={mode} fill={`var(${presetToken(mode, id)})`} picked={picked === id} onPick={() => onPick(color(id))} />
      ))}
      {divider}
      <AccentCustomSwatch mode={mode} custom={picked === 'custom' ? accent : ''} start={color('blue')} onPick={onPick} />
    </div>
  )
}
