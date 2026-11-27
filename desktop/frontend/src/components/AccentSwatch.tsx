import type { Appearance } from '../themes'
import type { ThemeName } from '../types'

// preview names the theme whose tokens the fill reads, for the swatch that shows a theme's own accent.
type AccentSwatchProps = { label: string; mode: Appearance; fill: string; preview?: ThemeName; picked: boolean; onPick(): void }

// AccentSwatch is one accent to pick, as a disc; a ring round it marks the one in use.
export function AccentSwatch({ label, mode, fill, preview, picked, onPick }: AccentSwatchProps) {
  return (
    <button
      type="button"
      title={label}
      aria-label={`${label} accent for ${mode} mode`}
      aria-pressed={picked}
      className="relative flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-full p-1 outline-none focus-visible:outline-2 focus-visible:outline-ink/60 focus-visible:outline-solid"
      onClick={onPick}
    >
      <span aria-hidden="true" data-preview={preview} className="block size-full rounded-full" style={{ backgroundColor: fill, boxShadow: `var(--shadow-preview-${mode})` }} />
      {picked && <span aria-hidden="true" className="pointer-events-none absolute inset-0 rounded-full ring-2 ring-ink ring-inset" />}
    </button>
  )
}
