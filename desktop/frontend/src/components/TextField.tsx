import { useState } from 'react'

type TextFieldProps = { label: string; value: string; placeholder?: string; onCommit(value: string): void }

// A one-line field that saves what is typed when it loses the focus or Enter is pressed, and goes back to the saved
// value on Esc.
export function TextField({ label, value, placeholder, onCommit }: TextFieldProps) {
  const [draft, setDraft] = useState<string | null>(null)
  const shown = draft ?? value

  const commit = () => {
    setDraft(null)
    if (shown.trim() !== value) onCommit(shown.trim())
  }

  return (
    <input
      aria-label={label}
      className="mono h-8 w-72 min-w-0 rounded-lg border border-line-strong bg-raised px-3 text-small text-ink placeholder:text-dim hover:bg-hover focus:bg-raised"
      value={shown}
      placeholder={placeholder}
      spellCheck={false}
      autoCapitalize="off"
      onChange={(e) => setDraft(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === 'Enter') e.currentTarget.blur()
        else if (e.key === 'Escape' && draft !== null) {
          e.stopPropagation()
          setDraft(null)
        }
      }}
    />
  )
}
