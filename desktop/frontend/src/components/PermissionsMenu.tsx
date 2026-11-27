import { ChevronUp } from 'lucide-react'
import { useRef, useState } from 'react'
import type { KeyboardEvent as ReactKeyboardEvent } from 'react'
import { ACCESSES, ACCESS_HINT, ACCESS_NAME } from '../access'
import type { Access } from '../types'
import { useDismiss } from '../useDismiss'
import { AccessIcon } from './AccessIcon'
import { SegmentedControl } from './SegmentedControl'

type PermissionsMenuProps = {
  access: Access
  // Why the access cannot change now, '' when it can.
  accessOff: string
  onAccess(access: Access): void
}

const OPTIONS = ACCESSES.map((value) => ({ value, label: ACCESS_NAME[value], icon: <AccessIcon access={value} /> }))

export function PermissionsMenu({ access, accessOff, onAccess }: PermissionsMenuProps) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)

  // Esc closes this popover and leaves the model menu open.
  useDismiss(root, open, () => setOpen(false))

  const arrows = (e: ReactKeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    const buttons = [...e.currentTarget.querySelectorAll<HTMLElement>('button:not(:disabled)')]
    const i = buttons.indexOf(document.activeElement as HTMLElement)
    buttons[Math.max(0, Math.min(i + (e.key === 'ArrowRight' ? 1 : -1), buttons.length - 1))]?.focus()
  }

  return (
    <div ref={root} className="relative ml-auto">
      <button
        ref={trigger}
        aria-label={`Permissions: ${ACCESS_NAME[access]}`}
        aria-haspopup="dialog"
        aria-expanded={open}
        className="flex h-7 items-center gap-1.5 rounded-md px-2 text-small text-soft hover:bg-[var(--bg-row-hover)] hover:text-ink aria-expanded:bg-[var(--bg-row-hover)] aria-expanded:text-ink focus-visible:outline-none"
        onMouseDown={(e) => e.preventDefault()}
        onClick={() => {
          if (!open && document.activeElement === trigger.current) requestAnimationFrame(() => root.current?.querySelector<HTMLElement>('[aria-pressed="true"]')?.focus())
          setOpen(!open)
        }}
      >
        <AccessIcon access={access} />
        {ACCESS_NAME[access]}
        <ChevronUp size={12} aria-hidden="true" />
      </button>
      {open && (
        <div role="dialog" aria-label="Permissions" className="fade-in absolute right-0 bottom-[calc(100%+0.5rem)] flex w-[17.5rem] flex-col gap-2 rounded-xl bg-[var(--bg-composer)] p-3 shadow-[0_0_0_1px_var(--border-strong),var(--shadow-popup)]" onKeyDown={arrows}>
          <p className="text-small text-soft">Permissions</p>
          <SegmentedControl
            label="Permissions"
            variant="track"
            shows="icon"
            options={OPTIONS}
            value={access}
            disabled={accessOff !== ''}
            onChange={(next) => {
              onAccess(next)
              setOpen(false)
              trigger.current?.focus()
            }}
          />
          <div>
            <p className="text-body font-medium">{ACCESS_NAME[access]}</p>
            <p className="text-small leading-tight text-soft">{ACCESS_HINT[access]}.</p>
            {accessOff && <p className="pt-1 text-small leading-tight text-idle">{accessOff}.</p>}
          </div>
        </div>
      )}
    </div>
  )
}
