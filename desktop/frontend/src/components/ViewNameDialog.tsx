import { useState } from 'react'
import { createPortal } from 'react-dom'
import { VIEW_NAME_MAX } from '../listViews'

type ViewNameDialogProps = {
  title: string
  placeholder: string
  initial: string
  error(name: string): string
  onSave(name: string): void
  onClose(): void
}

export function ViewNameDialog({ title, placeholder, initial, error, onSave, onClose }: ViewNameDialogProps) {
  const [name, setName] = useState(initial)
  const [touched, setTouched] = useState(false)
  const problem = error(name)
  const shown = touched && problem !== '' ? problem : ''

  const save = () => {
    setTouched(true)
    if (problem === '') onSave(name)
  }

  return createPortal(
    <div className="fade-in fixed inset-0" style={{ zIndex: 'var(--z-overlay)', background: 'var(--bg-overlay)' }} onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="absolute left-1/2 flex w-[min(24rem,92vw)] -translate-x-1/2 flex-col gap-3 rounded-md border border-line-strong bg-surface p-4 shadow-2xl"
        style={{ top: '12vh' }}
        onKeyDown={(e) => {
          e.stopPropagation()
          if (e.key === 'Escape') onClose()
        }}
      >
        <div className="flex flex-col gap-1">
          <h2 className="text-title font-semibold">{title}</h2>
          <p className="text-small text-soft">Save your search, filters, and sort order on this device.</p>
        </div>
        <label className="flex flex-col gap-1.5 text-small font-medium">
          View name
          <input
            autoFocus
            value={name}
            maxLength={VIEW_NAME_MAX + 10}
            placeholder={placeholder}
            aria-invalid={shown !== ''}
            className="field w-full font-normal"
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && save()}
          />
          {shown !== '' && <span className="text-label font-normal text-danger">{shown}</span>}
        </label>
        <div className="flex justify-end gap-2">
          <button className="btn" onClick={onClose}>
            Cancel
          </button>
          <button className="btn btn-primary" disabled={problem !== ''} onClick={save}>
            Save
          </button>
        </div>
      </div>
    </div>,
    document.body,
  )
}
