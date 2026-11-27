import { Star } from 'lucide-react'
import type { KeyboardEvent, ReactNode } from 'react'
import type { ModelTab } from '../modelMenu'
import { ClaudeMark } from './ClaudeMark'
import { moveFocus } from './moveFocus'

type ModelRailProps = {
  tab: ModelTab
  onTab(tab: ModelTab): void
  onSearch(): void
}

type RailButtonProps = { label: string; pressed: boolean; onClick(): void; children: ReactNode }

function RailButton({ label, pressed, onClick, children }: RailButtonProps) {
  return (
    <div className="relative">
      <button
        aria-label={label}
        title={label}
        aria-pressed={pressed}
        className="flex aspect-square w-full items-center justify-center rounded-md text-ink hover:bg-[var(--bg-row-hover)] focus-visible:bg-[var(--bg-row-hover)] focus-visible:outline-none"
        onMouseDown={(e) => e.preventDefault()}
        onClick={onClick}
      >
        {children}
      </button>
      {pressed && <span aria-hidden="true" className="pointer-events-none absolute top-1/2 right-0 h-5 w-[3px] -translate-y-1/2 rounded-full bg-[var(--accent)]" />}
    </div>
  )
}

export function ModelRail({ tab, onTab, onSearch }: ModelRailProps) {
  const onKeyDown = (e: KeyboardEvent<HTMLElement>) => {
    if (e.altKey || e.ctrlKey || e.metaKey) return
    if (e.key === 'ArrowRight') {
      e.preventDefault()
      onSearch()
    } else if (e.key.length === 1 && e.key.trim() !== '') onSearch()
    else moveFocus(e, 'button')
  }
  return (
    <nav aria-label="Providers" data-model-rail className="flex w-11 flex-none flex-col gap-1 bg-[var(--bg-segment)] p-1" onKeyDown={onKeyDown}>
      <RailButton label="Favorites" pressed={tab === 'favorites'} onClick={() => onTab('favorites')}>
        <Star size={18} fill="currentColor" aria-hidden="true" />
      </RailButton>
      <div className="border-b border-line" aria-hidden="true" />
      <RailButton label="Claude" pressed={tab === 'claude'} onClick={() => onTab('claude')}>
        <ClaudeMark size={20} />
      </RailButton>
    </nav>
  )
}
