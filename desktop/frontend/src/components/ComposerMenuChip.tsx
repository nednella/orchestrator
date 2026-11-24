import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { Icon } from './Icon'

type ComposerMenuChipProps = {
  label: string
  children: ReactNode
  menu(close: () => void): ReactNode
}

export function ComposerMenuChip({ label, children, menu }: ComposerMenuChipProps) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)

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
    }
  }, [open])

  return (
    <div ref={root} className="relative flex-none">
      <button className="composer-chip" aria-label={label} aria-haspopup="menu" aria-expanded={open} onMouseDown={(e) => e.preventDefault()} onClick={() => setOpen(!open)}>
        {children}
        <Icon name="chevron" size={12} />
      </button>
      {open && (
        <div className="composer-menu fade-in right-0" role="menu" aria-label={label} onMouseDown={(e) => e.preventDefault()}>
          {menu(() => setOpen(false))}
        </div>
      )}
    </div>
  )
}
