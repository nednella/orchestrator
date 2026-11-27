import type { KeyboardEvent, RefObject } from 'react'
import type { ModelTab } from '../modelMenu'
import { Icon } from './Icon'
import { ModelListRow, modelOptionId } from './ModelListRow'

type ModelChoicesProps = {
  tab: ModelTab
  query: string
  list: string[]
  highlighted: string
  selected: string
  favorites: string[]
  off: string
  searchRef: RefObject<HTMLInputElement>
  onQuery(query: string): void
  onHighlight(id: string): void
  onPick(id: string): void
  onStar(id: string): void
  // onKey is the search field's keys that leave the list: Left, Right and Enter.
  onKey(e: KeyboardEvent<HTMLInputElement>): void
}

export function ModelChoices({ tab, query, list, highlighted, selected, favorites, off, searchRef, onQuery, onHighlight, onPick, onStar, onKey }: ModelChoicesProps) {
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const i = list.indexOf(highlighted) + (e.key === 'ArrowDown' ? 1 : -1)
      onHighlight(list[Math.max(0, Math.min(i, list.length - 1))] ?? '')
      return
    }
    if (e.key === 'Enter' && highlighted && !off) {
      e.preventDefault()
      onPick(highlighted)
      return
    }
    onKey(e)
  }

  return (
    <div className="flex min-w-0 flex-1 flex-col">
      <div className="flex-none px-3 pt-2.5">
        <label className="flex h-8 items-center gap-2 border-b border-line pb-1.5 text-body">
          <span className="flex text-dim">
            <Icon name="search" size={14} />
          </span>
          <input
            ref={searchRef}
            autoFocus
            role="combobox"
            aria-expanded="true"
            aria-controls="model-options"
            aria-activedescendant={highlighted ? modelOptionId(highlighted) : undefined}
            aria-label="Search models"
            placeholder="Search models..."
            value={query}
            spellCheck={false}
            className="min-w-0 flex-1 bg-transparent outline-none placeholder:text-dim"
            onChange={(e) => onQuery(e.target.value)}
            onKeyDown={onKeyDown}
          />
        </label>
      </div>
      <div id="model-options" role="listbox" aria-label="Models" className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2 pt-2.5 pb-1.5">
        {list.map((id, i) => (
          <ModelListRow
            key={id}
            id={id}
            highlighted={id === highlighted}
            selected={id === selected}
            favorite={favorites.includes(id)}
            starOnHover={tab === 'favorites' && query === ''}
            jump={off ? null : i + 1}
            off={off}
            onPick={() => onPick(id)}
            onStar={() => onStar(id)}
            onHover={() => onHighlight(id)}
          />
        ))}
        {list.length === 0 && <p className="px-2 py-1.5 text-small text-dim">{tab === 'favorites' && query === '' ? 'No favorites yet' : 'No models found'}</p>}
      </div>
    </div>
  )
}
