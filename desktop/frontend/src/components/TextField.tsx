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
      className="setting-field h-7 w-full min-w-0 px-2.5 text-ink placeholder:text-dim @lg:w-72"
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
