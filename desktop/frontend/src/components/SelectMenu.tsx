import { ChevronDown } from 'lucide-react'
import { useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { SelectPopup } from './SelectPopup'

export type SelectOption<T extends string> = { value: T; label: string; icon?: ReactNode; description?: string }

type SelectMenuProps<T extends string> = {
  label: string
  options: SelectOption<T>[]
  value: T
  onChange(value: T): void
}

// A select whose list is its own popup. The trigger shows the icon of the chosen option when it has one; the list
// marks the chosen option by its fill, with no check.
export function SelectMenu<T extends string>({ label, options, value, onChange }: SelectMenuProps<T>) {
  const trigger = useRef<HTMLButtonElement>(null)
  const [anchor, setAnchor] = useState<DOMRect | null>(null)
  const chosen = options.find((option) => option.value === value) ?? options[0]

  const close = (refocus: boolean) => {
    setAnchor(null)
    if (refocus) trigger.current?.focus()
  }

  return (
    <>
      <button
        ref={trigger}
        type="button"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={anchor !== null}
        className="setting-field inline-flex min-h-7 w-full cursor-pointer items-center justify-between gap-1.5 px-2.5 text-left @lg:w-auto @lg:min-w-40"
        onClick={() => setAnchor(anchor ? null : (trigger.current?.getBoundingClientRect() ?? null))}
      >
        <span className="flex min-w-0 flex-1 items-center gap-1.5">
          {chosen.icon && <span className="flex-none text-soft">{chosen.icon}</span>}
          <span className="truncate">{chosen.label}</span>
        </span>
        <ChevronDown size={14} aria-hidden="true" className="-me-1 flex-none text-soft opacity-70" />
      </button>
      {anchor && (
        <SelectPopup
          anchor={anchor}
          trigger={trigger}
          label={label}
          options={options}
          value={value}
          onPick={(picked) => {
            close(true)
            if (picked !== value) onChange(picked)
          }}
          onClose={close}
        />
      )}
    </>
  )
}
