import { Square, SquareSplitHorizontal, SquareTerminal, X } from 'lucide-react'
import type { ReactNode } from 'react'
import { showsGroupHeaders, terminalLabel } from '../terminalGroups'
import type { ThreadTerms } from '../terminalGroups'
import type { Shell } from '../types'

type TerminalListProps = { terms: ThreadTerms; shells: Shell[]; buttons: ReactNode; onSelect(id: string): void; onClose(id: string): void }

// TerminalList lists the thread's terminals by group, each named by the process in front in it.
export function TerminalList({ terms, shells, buttons, onSelect, onClose }: TerminalListProps) {
  const headers = showsGroupHeaders(terms)
  const shown = terms.groups.find((g) => g.id === terms.active)
  const commandOf = (id: string) => shells.find((s) => s.id === id)?.command ?? ''
  let n = 0

  return (
    <div className="ml-1.5 flex w-[144px] flex-none flex-col border-l border-(--border-terminal-soft) bg-(--bg-terminal-list)">
      {buttons}
      <div className="min-h-0 flex-1 overflow-y-auto p-1">
        {terms.groups.map((g) => (
          <div key={g.id}>
            {headers && (
              <button
                className={`flex h-[22px] w-full items-center gap-1.5 rounded-md px-1.5 text-[11px] leading-4 text-dim ${g === shown ? 'bg-(--bg-terminal-group)' : ''}`}
                onClick={() => onSelect(g.active)}
              >
                {g.panes.length > 1 ? <SquareSplitHorizontal size={12} aria-hidden="true" /> : <Square size={12} aria-hidden="true" />}
                <span className="truncate">{g.panes.length > 1 ? 'Side by side' : 'Single'}</span>
                <span className="text-[10px] text-(--text-terminal-count)">{g.panes.length}</span>
              </button>
            )}
            {g.panes.map((id) => {
              const label = terminalLabel(commandOf(id), ++n)
              const active = g === shown && id === g.active
              return (
                <div
                  key={id}
                  className={`group/row flex h-6 items-center gap-1.5 rounded-md pr-2 pl-1.5 text-[12px] ${active ? 'bg-hover text-ink' : 'text-dim hover:bg-(--bg-terminal-row-hover)'}`}
                  onClick={() => onSelect(id)}
                >
                  <span className="relative flex size-4 flex-none items-center justify-center">
                    <SquareTerminal size={12} className="group-hover/row:invisible" aria-hidden="true" />
                    <button
                      className="invisible absolute inset-0 flex items-center justify-center rounded-sm group-hover/row:visible hover:bg-hover focus-visible:visible"
                      aria-label={`Close ${label}`}
                      onClick={(e) => {
                        e.stopPropagation()
                        onClose(id)
                      }}
                    >
                      <X size={12} aria-hidden="true" />
                    </button>
                  </span>
                  <button className="min-w-0 flex-1 truncate text-left" aria-current={active}>
                    {label}
                  </button>
                </div>
              )
            })}
          </div>
        ))}
      </div>
    </div>
  )
}
