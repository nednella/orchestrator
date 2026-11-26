import { Check } from 'lucide-react'

type AddProjectCheckProps = { checked: boolean }

// The box at the end of a row that toggles; the row takes the click.
export function AddProjectCheck({ checked }: AddProjectCheckProps) {
  return (
    <span aria-hidden="true" className={`pointer-events-none flex size-4 flex-none items-center justify-center rounded-[4px] border ${checked ? 'border-accent bg-accent text-(--text-on-accent)' : 'border-line-strong bg-raised'}`}>
      {checked && <Check size={12} strokeWidth={3} />}
    </span>
  )
}
