import { Search, X } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import type { FocusTarget } from '../OrchestratorContext'

export type ListSearchProps = {
  value: string
  label: string
  placeholder: string
  title?: string
  // The focus request that puts the cursor here.
  focusOn?: FocusTarget
  onChange(value: string): void
  // Escape in an empty field.
  onLeave(): void
}

export function ListSearch({ value, label, placeholder, title, focusOn, onChange, onLeave }: ListSearchProps) {
  const { focusRequest } = useOrchestrator('focusRequest')
  const input = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (focusOn === undefined || focusRequest.target !== focusOn) return
    input.current?.focus()
    input.current?.select()
  }, [focusRequest, focusOn])

  return (
    <div className="relative min-w-0 basis-full @lg:basis-0 @lg:flex-1 @lg:min-w-48">
      <span className="pointer-events-none absolute inset-y-0 left-2.5 flex items-center text-soft">
        <Search size={16} />
      </span>
      <input
        ref={input}
        type="text"
        role="searchbox"
        value={value}
        spellCheck={false}
        placeholder={placeholder}
        aria-label={label}
        title={title}
        className="field h-8 w-full pr-8 pl-8 text-small"
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key !== 'Escape' || e.nativeEvent.isComposing) return
          e.stopPropagation()
          if (value) onChange('')
          else onLeave()
        }}
      />
      {value && (
        <button
          className="btn btn-ghost absolute top-1 right-1 h-6 w-6 justify-center px-0"
          aria-label="Clear the search"
          title="Clear"
          onClick={() => {
            onChange('')
            input.current?.focus()
          }}
        >
          <X size={13} />
        </button>
      )}
    </div>
  )
}
