import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import type { KeyboardEvent, ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { LIST_BUTTON } from '../listStyle'

const EDGE = 8

type ListMenuProps = {
  label: string
  title?: string
  align?: 'start' | 'end'
  className?: string
  trigger: ReactNode
  children(close: () => void, menuId: string): ReactNode
}

// moveFocus steps the focus through the items of the layer that holds it; the layer is the popup or one of its submenus.
export function moveFocus(e: KeyboardEvent<HTMLElement>, keys: Record<string, number | 'first' | 'last'>) {
  const step = keys[e.key]
  if (step === undefined) return false
  const layer = (e.target as HTMLElement).closest<HTMLElement>('[data-menu-layer]')
  const items = [...(layer?.querySelectorAll<HTMLElement>('[role^="menuitem"]') ?? [])].filter((el) => !el.hasAttribute('disabled') && el.closest('[data-menu-layer]') === layer)
  if (items.length === 0) return false
  e.preventDefault()
  const at = items.indexOf(document.activeElement as HTMLElement)
  const to = step === 'first' ? 0 : step === 'last' ? items.length - 1 : at + step
  items[(to + items.length) % items.length].focus()
  return true
}

// ListMenu is a button that opens a popup under it. The popup and the submenus its children open belong to menuId, so a
// press inside any of them is not a press outside.
export function ListMenu({ label, title, align = 'start', className = '', trigger, children }: ListMenuProps) {
  const [open, setOpen] = useState(false)
  const [at, setAt] = useState<{ left: number; top: number; maxHeight: number } | null>(null)
  const button = useRef<HTMLButtonElement>(null)
  const popup = useRef<HTMLDivElement>(null)
  const menuId = useId()

  const close = () => {
    setOpen(false)
    setAt(null)
  }

  useLayoutEffect(() => {
    if (!open || !popup.current || !button.current) return
    const from = button.current.getBoundingClientRect()
    const width = popup.current.getBoundingClientRect().width
    const left = align === 'end' ? from.right - width : from.left
    const top = from.bottom + 4
    setAt({ left: Math.max(EDGE, Math.min(left, window.innerWidth - width - EDGE)), top, maxHeight: window.innerHeight - top - EDGE })
  }, [open, align])

  useEffect(() => {
    if (!open) return
    popup.current?.querySelector<HTMLElement>('[role^="menuitem"]')?.focus()
    const outside = (e: MouseEvent) => {
      const target = e.target as Element
      if (!button.current?.contains(target) && !target.closest(`[data-menu-of="${CSS.escape(menuId)}"]`)) close()
    }
    const escape = (e: globalThis.KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopPropagation()
      close()
      button.current?.focus()
    }
    window.addEventListener('mousedown', outside, true)
    window.addEventListener('keydown', escape, true)
    window.addEventListener('resize', close)
    return () => {
      window.removeEventListener('mousedown', outside, true)
      window.removeEventListener('keydown', escape, true)
      window.removeEventListener('resize', close)
    }
  }, [open, menuId])

  return (
    <>
      <button ref={button} className={`${LIST_BUTTON} ${className}`} aria-label={label} title={title} aria-haspopup="menu" aria-expanded={open} onClick={() => (open ? close() : setOpen(true))}>
        {trigger}
      </button>
      {open &&
        createPortal(
          <div
            ref={popup}
            role="menu"
            aria-label={label}
            data-menu-layer
            data-menu-of={menuId}
            className="popup fade-in flex min-w-40 flex-col overflow-y-auto"
            style={at ? { left: at.left, top: at.top, maxHeight: at.maxHeight } : { left: 0, top: 0, opacity: 0 }}
            onKeyDown={(e) => {
              e.stopPropagation()
              if (e.key === 'Tab') close()
              else moveFocus(e, { ArrowDown: 1, ArrowUp: -1, Home: 'first', End: 'last' })
            }}
          >
            {children(close, menuId)}
          </div>,
          document.body,
        )}
    </>
  )
}
