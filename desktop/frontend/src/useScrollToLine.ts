import { useEffect } from 'react'
import type { RefObject } from 'react'
import type { FilePlace } from './filesView'

const FRAMES = 40

const renderedLines = (box: HTMLElement) => [...(box.querySelector('diffs-container')?.shadowRoot?.querySelectorAll<HTMLElement>('[data-line]') ?? [])]

// useScrollToLine scrolls the file code view in box to the line of a place, and keeps at it for a moment: the view draws
// only the lines near the scroll, so a line far off is not there until the scroll reaches it, and lines that wrap change
// the way as they are drawn.
export function useScrollToLine(box: RefObject<HTMLElement | null>, place: FilePlace | null) {
  useEffect(() => {
    if (place === null) return
    let frame = 0
    let tries = 0
    let settled = 0
    const step = () => {
      const root = box.current?.querySelector<HTMLElement>('.file-code')
      if (!root || ++tries > FRAMES) return
      const lines = renderedLines(root)
      const target = lines.find((el) => Number(el.dataset.line) === place.line)
      if (target) {
        const inView = target.getBoundingClientRect()
        const view = root.getBoundingClientRect()
        const centred = Math.abs(inView.top + inView.height / 2 - (view.top + view.height / 2)) < view.height / 4
        if (!centred) target.scrollIntoView({ block: 'center' })
        if (++settled >= 3) return
      } else if (lines.length > 0) {
        settled = 0
        const first = Number(lines[0].dataset.line)
        const last = Number(lines[lines.length - 1].dataset.line)
        const height = lines[lines.length - 1].getBoundingClientRect().bottom - lines[0].getBoundingClientRect().top
        const perLine = height / Math.max(1, last - first + 1)
        const middle = (first + last) / 2
        root.scrollTop += (place.line - middle) * perLine
      }
      frame = requestAnimationFrame(step)
    }
    frame = requestAnimationFrame(step)
    return () => cancelAnimationFrame(frame)
  }, [box, place])
}
