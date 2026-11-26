import { Plus, SquareSplitHorizontal, Trash2 } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Fragment, useState } from 'react'
import { formatShortcut } from '../actions'
import { NEW_TERMINAL_KEY, SPLIT_TERMINAL_KEY } from '../actions/shell'
import { MAX_PANES } from '../terminalGroups'
import { useDelayedTip } from '../useDelayedTip'
import { FloatingTip } from './FloatingTip'

type TerminalButtonsProps = { floating: boolean; full: boolean; onSplit(): void; onNew(): void; onClose(): void }

type Button = { id: string; icon: LucideIcon; label: string; off?: boolean; run(): void }

const FLOATING = 'absolute top-2 right-2 z-20 flex items-center overflow-hidden rounded-md border border-(--border-terminal) bg-surface shadow-(--shadow-pill)'
const HEADER = 'flex h-[22px] flex-none items-stretch [&>button+button]:border-l [&>button+button]:border-(--border-terminal-soft)'

// TerminalButtons split, add and close terminals: floating over a lone terminal, or as the list's header row.
export function TerminalButtons({ floating, full, onSplit, onNew, onClose }: TerminalButtonsProps) {
  const { anchor, tipHandlers } = useDelayedTip()
  const [tip, setTip] = useState('')
  const buttons: Button[] = [
    {
      id: 'split',
      icon: SquareSplitHorizontal,
      label: `Split terminal side by side (${formatShortcut(SPLIT_TERMINAL_KEY)})${full ? ` (${MAX_PANES} at most per group)` : ''}`,
      off: full,
      run: onSplit,
    },
    { id: 'new', icon: Plus, label: `New terminal (${formatShortcut(NEW_TERMINAL_KEY)})`, run: onNew },
    { id: 'close', icon: Trash2, label: 'Close terminal', run: onClose },
  ]

  return (
    <div className={floating ? FLOATING : HEADER}>
      {buttons.map(({ id, icon: Icon, label, off, run }, i) => (
        <Fragment key={id}>
          {floating && i > 0 && <span className="h-4 w-px flex-none bg-(--border-terminal)" />}
          <button
            className={`flex flex-1 items-center justify-center p-1 text-(--text-terminal-button) hover:bg-hover ${off ? 'cursor-not-allowed opacity-[0.64]' : ''}`}
            aria-label={label}
            aria-disabled={off}
            onClick={() => !off && run()}
            {...tipHandlers}
            onMouseEnter={(e) => {
              setTip(label)
              tipHandlers.onMouseEnter(e)
            }}
            onFocus={(e) => {
              setTip(label)
              tipHandlers.onFocus(e)
            }}
          >
            <Icon size={13} aria-hidden="true" />
          </button>
        </Fragment>
      ))}
      {anchor && <FloatingTip anchor={anchor} text={tip} below />}
    </div>
  )
}
