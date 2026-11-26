import { SquareTerminal, X } from 'lucide-react'
import type { ReactNode } from 'react'
import { folderName } from '../terminalGroups'
import type { TermGroup } from '../terminalGroups'
import { labelOf } from '../projectTerminalOps'
import type { Shell } from '../types'

type TerminalListProps = { group: TermGroup; shells: Shell[]; buttons: ReactNode; onSelect(id: string): void; onClose(id: string): void }

// TerminalList lists the group's terminals, each named by the process in front in it and the folder it started in.
export function TerminalList({ group, shells, buttons, onSelect, onClose }: TerminalListProps) {
  return (
    <div className="ml-1.5 flex w-[144px] flex-none flex-col border-l border-(--border-terminal-soft) bg-(--bg-terminal-list)">
      {buttons}
      <div className="min-h-0 flex-1 overflow-y-auto p-1">
        {group.panes.map((id) => {
          const label = labelOf(shells, id)
          const folder = shells.find((s) => s.id === id)?.folder ?? ''
          const active = id === group.active
          return (
            <div
              key={id}
              title={folder || undefined}
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
              <button className="flex min-w-0 flex-1 items-baseline gap-1 text-left" aria-current={active}>
                <span className="flex-none">{label}</span>
                {folder && <span className="min-w-0 truncate text-[11px] text-(--text-terminal-count)">{folderName(folder)}</span>}
              </button>
            </div>
          )
        })}
      </div>
    </div>
  )
}
