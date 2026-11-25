import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { Icon } from './Icon'

// disabled is why the menu cannot open now, shown as the chip's tooltip.
type ComposerMenuChipProps = {
  label: string
  disabled?: string
  children: ReactNode
  menu(close: () => void): ReactNode
}

export function ComposerMenuChip({ label, disabled, children, menu }: ComposerMenuChipProps) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  // A menu with its own field takes the focus; it goes back where it was, the composer, when the menu closes.
  const returnTo = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!open) return
    const outside = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false)
    }
    const escape = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopPropagation()
      setOpen(false)
    }
    window.addEventListener('mousedown', outside)
    window.addEventListener('keydown', escape, true)
    return () => {
      window.removeEventListener('mousedown', outside)
      window.removeEventListener('keydown', escape, true)
      if (document.activeElement === document.body || !document.activeElement) returnTo.current?.focus()
    }
  }, [open])

  return (
    <div ref={root} className="relative flex-none">
      <button
        className="composer-chip"
        aria-label={label}
        aria-haspopup="dialog"
        aria-expanded={open}
        disabled={Boolean(disabled)}
        title={disabled}
        onMouseDown={(e) => e.preventDefault()}
        onClick={() => {
          if (!open) returnTo.current = document.activeElement as HTMLElement | null
          setOpen(!open)
        }}
      >
        {children}
        <Icon name="chevron" size={12} />
      </button>
      {open && !disabled && (
        <div className="composer-menu fade-in right-0" role="dialog" aria-label={label} onMouseDown={(e) => e.preventDefault()}>
          {menu(() => setOpen(false))}
        </div>
      )}
    </div>
  )
}
