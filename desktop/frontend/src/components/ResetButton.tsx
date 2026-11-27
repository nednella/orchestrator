import { Undo2 } from 'lucide-react'

type ResetButtonProps = { label: string; onReset(): void }

export function ResetButton({ label, onReset }: ResetButtonProps) {
  return (
    <button type="button" aria-label={label} title="Reset to default" className="inline-flex size-5 items-center justify-center rounded-sm text-dim hover:bg-(--bg-row-hover) hover:text-ink" onClick={onReset}>
      <Undo2 size={12} aria-hidden="true" />
    </button>
  )
}
