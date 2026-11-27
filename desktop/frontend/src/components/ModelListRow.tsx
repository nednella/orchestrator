import { Check, Star } from 'lucide-react'
import { CLAUDE_MODELS } from '../model'
import { ClaudeMark } from './ClaudeMark'

type ModelListRowProps = {
  id: string
  highlighted: boolean
  selected: boolean
  favorite: boolean
  // In the Favourites tab every row is starred, so the star shows only on hover.
  starOnHover: boolean
  // The number of the shortcut that picks this row, null for none.
  jump: number | null
  // Why the row cannot be picked, '' when it can.
  off: string
  onPick(): void
  onStar(): void
  onHover(): void
}

export const modelOptionId = (id: string) => `model-option-${id}`

export function ModelListRow({ id, highlighted, selected, favorite, starOnHover, jump, off, onPick, onStar, onHover }: ModelListRowProps) {
  const name = CLAUDE_MODELS[id].name
  const starShown = favorite && !starOnHover
  return (
    <div
      id={modelOptionId(id)}
      role="option"
      aria-selected={selected}
      aria-disabled={off !== '' || undefined}
      title={off || undefined}
      data-highlighted={highlighted || undefined}
      className={`group flex h-8 flex-none items-center gap-2 rounded-md px-2 text-body data-[highlighted]:bg-[var(--bg-row-hover)] ${off ? 'cursor-not-allowed opacity-64' : 'cursor-pointer'} ${off && selected ? '!opacity-100' : ''}`}
      onMouseDown={(e) => e.preventDefault()}
      onMouseMove={onHover}
      onClick={off ? undefined : onPick}
    >
      <ClaudeMark size={16} />
      <span className="min-w-0 truncate">{name}</span>
      {selected && <Check size={13} aria-hidden="true" />}
      <span className="min-w-2 flex-1" />
      <button
        aria-label={favorite ? `Remove ${name} from favorites` : `Add ${name} to favorites`}
        title={favorite ? 'Remove from favorites' : 'Add to favorites'}
        tabIndex={-1}
        disabled={off !== ''}
        className={`flex size-6 flex-none items-center justify-center rounded-md text-soft hover:text-ink ${starShown ? '' : 'opacity-0 group-hover:opacity-100 group-data-[highlighted]:opacity-100'}`}
        onClick={(e) => {
          e.stopPropagation()
          onStar()
        }}
      >
        <Star size={13} aria-hidden="true" fill={favorite ? 'currentColor' : 'none'} style={favorite ? { color: 'var(--idle)' } : undefined} />
      </button>
      {jump !== null && <span className="flex-none rounded-md bg-[var(--bg-key-badge)] px-1.5 py-px text-label text-soft">⌘{jump}</span>}
    </div>
  )
}
