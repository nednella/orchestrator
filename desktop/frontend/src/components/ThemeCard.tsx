import { Moon } from 'lucide-react'
import type { DarkTheme } from '../types'

type ThemeCardProps = { theme: DarkTheme; label: string; active: boolean; onUse(): void }

// A card is one dark theme: a disc of its colours above its name. Picking it makes it the dark theme.
export function ThemeCard({ theme, label, active, onUse }: ThemeCardProps) {
  return (
    <div
      className={`cursor-pointer overflow-hidden rounded-xl border border-(--border-rule) transition-colors ${active ? 'bg-(--bg-row-active) shadow-[inset_0_0_0_1px_var(--accent)]' : 'bg-(--bg-card) hover:bg-(--bg-row-hover)'}`}
      onClick={onUse}
    >
      <div className="flex min-h-16 items-center justify-center px-3 pt-3">
        <span data-preview={theme} aria-hidden="true" className="relative flex size-17 items-center justify-center rounded-full p-1.5">
          <span
            className="size-14 rounded-full border-2 border-surface"
            style={{
              backgroundColor: 'var(--bg-app)',
              backgroundImage: 'radial-gradient(circle at 30% 25%, var(--accent), transparent 70%), radial-gradient(circle at 78% 82%, var(--bg-hover), transparent 65%)',
              boxShadow: 'inset 0 0 0 1px color-mix(in srgb, var(--text) 14%, transparent)',
            }}
          />
          {active && (
            <>
              <span className="pointer-events-none absolute inset-0 rounded-full ring-2 ring-(--accent) ring-inset" />
              <span className="pointer-events-none absolute right-0.5 bottom-0.5 flex size-5 items-center justify-center rounded-full border border-(--border-rule) bg-app text-ink shadow-sm">
                <Moon size={12} aria-hidden="true" />
              </span>
            </>
          )}
        </span>
      </div>
      <div className="px-3 pt-2 pb-3">
        <button type="button" aria-label={`Use ${label} theme`} aria-pressed={active} className="block max-w-full truncate text-left text-body font-medium">
          {label}
        </button>
      </div>
    </div>
  )
}
