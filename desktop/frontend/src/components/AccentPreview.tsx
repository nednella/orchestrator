import type { CSSProperties } from 'react'
import { textOnAccent } from '../accent'
import type { ThemeName } from '../types'

type AccentPreviewProps = { theme: ThemeName; accent: string }

// AccentPreview shows the accent as a button and as a focus ring, on the surface of the theme of the appearance it is for.
export function AccentPreview({ theme, accent }: AccentPreviewProps) {
  const style = { '--accent': accent, '--text-on-accent': `var(${textOnAccent(accent)})` } as CSSProperties
  return (
    <div data-preview={theme} aria-hidden="true" className="flex items-center gap-3 rounded-lg border border-(--border-rule) bg-(--bg-surface) p-3 text-ink" style={style}>
      <span className="inline-flex h-7 items-center rounded-md bg-(--accent) px-3 text-small font-medium text-(--text-on-accent)">Button</span>
      <span className="inline-flex h-7 items-center rounded-md px-3 text-small ring-2 ring-(--accent)">Focus</span>
    </div>
  )
}
