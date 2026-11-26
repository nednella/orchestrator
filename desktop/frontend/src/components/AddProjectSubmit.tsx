import { PaletteKey } from './PaletteKey'

type AddProjectSubmitProps = { label: string; keys: string; disabled: boolean; onSubmit(): void }

// The button at the right edge of the input; the key it shows is the one that does the same.
export function AddProjectSubmit({ label, keys, disabled, onSubmit }: AddProjectSubmitProps) {
  return (
    <button
      type="button"
      className="btn absolute top-1/2 right-2.5 h-6 -translate-y-1/2 gap-2 px-[calc(0.5rem-1px)] text-small disabled:opacity-60"
      title={`${label} (${keys})`}
      aria-label={`${label} (${keys})`}
      disabled={disabled}
      onMouseDown={(e) => e.preventDefault()}
      onClick={onSubmit}
    >
      <span>{label}</span>
      <span className="pointer-events-none -me-0.5">
        <PaletteKey>{keys}</PaletteKey>
      </span>
    </button>
  )
}
