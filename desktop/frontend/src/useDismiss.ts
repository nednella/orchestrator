import { useEffect, useLayoutEffect, useRef } from 'react'
import type { RefObject } from 'react'

// useDismiss calls close at a click outside root, which holds a button and what it opened, or at Esc, which it takes
// before the page hears it. It listens from the open on, so a listener added after it hears Esc after it.
export function useDismiss(root: RefObject<HTMLElement | null>, open: boolean, close: () => void) {
  const latest = useRef(close)
  useLayoutEffect(() => {
    latest.current = close
  })
  useEffect(() => {
    if (!open) return
    const outside = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) latest.current()
    }
    const escape = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopPropagation()
      latest.current()
    }
    window.addEventListener('mousedown', outside)
    window.addEventListener('keydown', escape, true)
    return () => {
      window.removeEventListener('mousedown', outside)
      window.removeEventListener('keydown', escape, true)
    }
  }, [root, open])
}
