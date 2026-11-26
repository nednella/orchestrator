import { useLayoutEffect, useState } from 'react'
import type { RefObject } from 'react'

// useFitsWithin is whether content, at its full width, fits in the width of room, kept as either one resizes.
export function useFitsWithin(room: RefObject<HTMLElement | null>, content: RefObject<HTMLElement | null>): boolean {
  const [fits, setFits] = useState(true)
  useLayoutEffect(() => {
    const roomEl = room.current
    const contentEl = content.current
    if (!roomEl || !contentEl) return
    const measure = () => setFits(contentEl.offsetWidth <= roomEl.clientWidth)
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(roomEl)
    observer.observe(contentEl)
    return () => observer.disconnect()
  }, [room, content])
  return fits
}
