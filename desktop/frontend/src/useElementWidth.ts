import { useLayoutEffect, useState } from 'react'
import type { RefObject } from 'react'

// useElementWidth is the width of el in px, kept as it resizes, 0 before it is measured.
export function useElementWidth(el: RefObject<HTMLElement | null>): number {
  const [width, setWidth] = useState(0)
  useLayoutEffect(() => {
    const node = el.current
    if (!node) return
    const measure = () => setWidth(node.clientWidth)
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(node)
    return () => observer.disconnect()
  }, [el])
  return width
}
