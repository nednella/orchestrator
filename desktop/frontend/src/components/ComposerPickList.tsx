import { useRef } from 'react'
import type { ReactNode } from 'react'
import { useScrollCursorIntoView } from '../useScrollCursorIntoView'

export type PickRow = { key: string; icon: ReactNode; name: string; description?: string; badge?: string }

type ComposerPickListProps = { label: string; rows: PickRow[]; cursor: number; empty: string; onPick(index: number): void }

// The composer keeps the focus and moves the cursor: Up and Down move it, Enter or Tab picks, Esc closes the list.
export function ComposerPickList({ label, rows, cursor, empty, onPick }: ComposerPickListProps) {
  const list = useRef<HTMLDivElement>(null)
  useScrollCursorIntoView(list, cursor)

  if (rows.length === 0) return <p className="px-2 py-1.5 text-label text-dim">{empty}</p>
  return (
    <div ref={list} role="listbox" aria-label={label} className="flex max-h-72 flex-col gap-0.5 overflow-y-auto">
      {rows.map((row, i) => (
        <button
          key={row.key}
          role="option"
          aria-selected={i === cursor}
          data-selected={i === cursor}
          data-cursor={i === cursor}
          className="composer-option flex-none"
          onMouseDown={(e) => {
            e.preventDefault()
            onPick(i)
          }}
        >
          {row.icon}
          <span className={`min-w-0 truncate text-label font-medium ${row.description ? 'max-w-[60%] flex-none' : 'flex-1'}`}>{row.name}</span>
          {row.description && <span className="min-w-0 flex-1 truncate text-label text-dim">{row.description}</span>}
          {row.badge && <span className="flex-none text-label text-dim">{row.badge}</span>}
        </button>
      ))}
    </div>
  )
}
