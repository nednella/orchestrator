import { Bot, Check, User } from 'lucide-react'
import type { Evidence } from '../types'
import { Ago } from './Ago'

type EvidenceCardProps = {
  item: Evidence
  selectable: boolean
  selected: boolean
  onOpen(): void
  onToggle(): void
}

export function EvidenceCard({ item, selectable, selected, onOpen, onToggle }: EvidenceCardProps) {
  const image = item.kind === 'image'
  const Source = item.source === 'agent' ? Bot : User

  return (
    <article className={`flex min-w-0 flex-col overflow-hidden rounded-xl border bg-(--bg-card) ${selected ? 'border-(--accent) ring-1 ring-(--accent)' : 'border-(--border-rule)'}`}>
      {image ? (
        <button
          className="relative block aspect-[16/10] w-full bg-term"
          title={selectable ? 'Select for comparison' : 'Open larger'}
          aria-pressed={selectable ? selected : undefined}
          onClick={selectable ? onToggle : onOpen}
        >
          <img src={item.url} alt={item.caption || 'Image'} className="h-full w-full object-cover" />
          {selectable && (
            <span className={`absolute top-2 right-2 grid size-5 place-items-center rounded-full border ${selected ? 'border-(--accent) bg-(--accent) text-(--text-on-accent)' : 'border-(--border-strong) bg-(--bg-app)'}`}>
              {selected && <Check size={12} aria-hidden="true" />}
            </span>
          )}
        </button>
      ) : (
        <div className="max-h-48 overflow-y-auto px-3 pt-3 text-body break-words whitespace-pre-wrap">{item.text}</div>
      )}
      <div className="flex flex-1 flex-col gap-1.5 px-3 py-2.5">
        {item.caption && <p className="text-small text-soft">{item.caption}</p>}
        <p className="mt-auto flex items-center gap-1.5 text-label text-dim">
          <Source size={12} aria-hidden="true" />
          <span>{item.source === 'agent' ? 'Agent' : 'You'}</span>
          <span aria-hidden="true">·</span>
          <span><Ago at={item.at} /></span>
        </p>
      </div>
    </article>
  )
}
