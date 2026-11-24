import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { CSSProperties, RefObject } from 'react'

export type Point = { x: number; y: number }

const EDGE_PX = 4

// Places a fixed popup at a point and keeps it inside the window, flipping it above the point when it would run
// off the bottom. Escape, a click outside and a resize close it; focus goes back where it was if it was left nowhere.
export function useFloatingAt(popup: RefObject<HTMLElement | null>, at: Point, onClose: () => void): CSSProperties {
  const [place, setPlace] = useState<Point | null>(null)
  const close = useRef(onClose)

  useLayoutEffect(() => {
    close.current = onClose
  })

  useLayoutEffect(() => {
    const el = popup.current
    if (!el) return
    const { width, height } = el.getBoundingClientRect()
    const x = Math.max(EDGE_PX, Math.min(at.x, window.innerWidth - width - EDGE_PX))
    const below = at.y + height <= window.innerHeight - EDGE_PX
    const y = below ? at.y : Math.max(EDGE_PX, at.y - height)
    setPlace({ x, y })
  }, [popup, at])

  useEffect(() => {
    const before = document.activeElement as HTMLElement | null
    const outside = (e: MouseEvent) => {
      if (!popup.current?.contains(e.target as Node)) close.current()
    }
    const escape = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopPropagation()
      close.current()
    }
    window.addEventListener('mousedown', outside, true)
    window.addEventListener('keydown', escape, true)
    const resize = () => close.current()
    window.addEventListener('resize', resize)
    return () => {
      window.removeEventListener('mousedown', outside, true)
      window.removeEventListener('keydown', escape, true)
      window.removeEventListener('resize', resize)
      if (document.activeElement === document.body || !document.activeElement) before?.focus()
    }
  }, [popup])

  return place ? { left: place.x, top: place.y } : { left: at.x, top: at.y, opacity: 0 }
}
