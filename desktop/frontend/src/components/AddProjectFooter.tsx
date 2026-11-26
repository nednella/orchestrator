import { ArrowDown, ArrowUp } from 'lucide-react'
import type { ReactNode } from 'react'
import { PaletteKey } from './PaletteKey'

type AddProjectFooterProps = { action?: string; trailing?: ReactNode }

export function AddProjectFooter({ action, trailing }: AddProjectFooterProps) {
  return (
    <div className="flex items-center justify-between gap-2 border-t border-(--border-palette-foot) bg-(--bg-palette-foot) px-4 py-2.5 text-body font-medium text-soft">
      <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1.5">
        <span className="inline-flex items-center gap-1.5">
          <PaletteKey>
            <ArrowUp size={12} aria-hidden="true" />
          </PaletteKey>
          <PaletteKey>
            <ArrowDown size={12} aria-hidden="true" />
          </PaletteKey>
          <span>Navigate</span>
        </span>
        {action !== undefined && (
          <span className="inline-flex items-center gap-1.5">
            <PaletteKey>Enter</PaletteKey>
            <span>{action}</span>
          </span>
        )}
        <span className="inline-flex items-center gap-1.5">
          <PaletteKey>Backspace</PaletteKey>
          <span>Back</span>
        </span>
        <span className="inline-flex items-center gap-1.5">
          <PaletteKey>Esc</PaletteKey>
          <span>Close</span>
        </span>
      </div>
      {trailing}
    </div>
  )
}
