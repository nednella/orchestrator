import type { Appearance } from '../themes'
import type { ThemeName } from '../types'

type ThemePreviewCircleProps = { theme: ThemeName; mode: Appearance }

// Where each glow sits and how far it reaches: a light disc glows from its top right, a dark one from its bottom left.
const PAINT = {
  light: { accentAt: '72% 22%', accentMid: 72, surfaceAt: '18% 82%' },
  dark: { accentAt: '28% 78%', accentMid: 62, surfaceAt: '82% 18%' },
}

// ThemePreviewCircle is a theme as a disc: its canvas, its accent and its hover surface, blurred into each other.
export function ThemePreviewCircle({ theme, mode }: ThemePreviewCircleProps) {
  const { accentAt, accentMid, surfaceAt } = PAINT[mode]
  return (
    <span aria-hidden="true" className="relative block size-14 shrink-0 overflow-hidden rounded-full border-2 border-app" style={{ boxShadow: `var(--shadow-preview-${mode})` }}>
      <span
        data-preview={theme}
        className="absolute inset-0 scale-110 rounded-full blur-[3px]"
        style={{
          backgroundColor: `color-mix(in oklab, var(--bg-app) 80%, var(--preview-${mode}-base))`,
          backgroundImage: [
            `radial-gradient(circle at ${accentAt} in oklab, var(--accent) 0%, color-mix(in oklab, var(--accent) ${accentMid}%, transparent) 28%, transparent 58%)`,
            `radial-gradient(circle at ${surfaceAt} in oklab, color-mix(in oklab, var(--bg-hover) 45%, transparent) 0%, transparent 55%)`,
          ].join(', '),
        }}
      />
    </span>
  )
}
