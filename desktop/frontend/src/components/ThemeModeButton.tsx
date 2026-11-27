import { Moon, Sun } from 'lucide-react'
import { modeTip, type ThemePreview } from '../themes'
import { useDelayedTip } from '../useDelayedTip'
import { FloatingTip } from './FloatingTip'
import { ThemePreviewCircle } from './ThemePreviewCircle'

type ThemeModeButtonProps = { label: string; preview: ThemePreview; picked: boolean; onPick(): void; onHover(over: boolean): void }

// ThemeModeButton is one appearance of a theme card: picking it makes the theme that appearance's, and a ring and a
// sun or a moon mark it while it is.
export function ThemeModeButton({ label, preview, picked, onPick, onHover }: ThemeModeButtonProps) {
  const { anchor, tipHandlers } = useDelayedTip()
  const { mode, theme } = preview
  return (
    <button
      type="button"
      aria-label={`Use ${label} ${mode} mode`}
      aria-pressed={picked}
      className={`relative flex size-17 shrink-0 transform-gpu cursor-pointer items-center justify-center rounded-full p-1.5 transition-transform outline-none focus-visible:outline-2 focus-visible:-outline-offset-5 focus-visible:outline-ink/60 focus-visible:outline-solid ${picked ? '' : 'hover:scale-105'}`}
      {...tipHandlers}
      onMouseEnter={(e) => {
        tipHandlers.onMouseEnter(e)
        onHover(true)
      }}
      onMouseLeave={() => {
        tipHandlers.onMouseLeave()
        onHover(false)
      }}
      onClick={(e) => {
        e.stopPropagation()
        onPick()
      }}
    >
      <ThemePreviewCircle theme={theme} mode={mode} />
      {picked && (
        <>
          <span aria-hidden="true" className="pointer-events-none absolute inset-0 rounded-full ring-2 ring-(--accent) ring-inset" />
          <span aria-hidden="true" className="pointer-events-none absolute right-0.5 bottom-0.5 flex size-5 items-center justify-center rounded-full border border-(--border-rule) bg-app text-ink shadow-sm">
            {mode === 'light' ? <Sun size={12} /> : <Moon size={12} />}
          </span>
        </>
      )}
      {anchor && <FloatingTip anchor={anchor} text={modeTip(mode)} />}
    </button>
  )
}
