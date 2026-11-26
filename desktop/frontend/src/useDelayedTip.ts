import { useEffect, useRef, useState } from 'react'
import type { FocusEvent, MouseEvent } from 'react'

// useDelayedTip shows a tooltip for the element a pointer rests on, or the keyboard focuses, after delay ms; a press
// or leaving it hides the tip. anchor is where the element is while the tip shows.
export function useDelayedTip(delay = 600) {
  const [anchor, setAnchor] = useState<DOMRect | null>(null)
  const timer = useRef(0)

  useEffect(() => () => clearTimeout(timer.current), [])

  const show = (el: HTMLElement) => {
    clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setAnchor(el.getBoundingClientRect()), delay)
  }
  const hide = () => {
    clearTimeout(timer.current)
    setAnchor(null)
  }

  return {
    anchor,
    tipHandlers: {
      onMouseEnter: (e: MouseEvent<HTMLElement>) => show(e.currentTarget),
      onMouseLeave: hide,
      onMouseDown: hide,
      onFocus: (e: FocusEvent<HTMLElement>) => e.currentTarget.matches(':focus-visible') && show(e.currentTarget),
      onBlur: hide,
    },
  }
}
