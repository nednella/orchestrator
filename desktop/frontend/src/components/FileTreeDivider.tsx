import { useRef } from 'react'
import type { KeyboardEvent, PointerEvent } from 'react'

const KEY_STEP = 16

type FileTreeDividerProps = { width: number; min: number; max: number; onWidth(width: number, commit: boolean): void }

// FileTreeDivider is the edge between the open file and the tree beside it; dragging it left, or the arrow keys, widens
// the tree. Home makes the tree as narrow as it can be and End as wide.
export function FileTreeDivider({ width, min, max, onWidth }: FileTreeDividerProps) {
  const drag = useRef<{ x: number; width: number } | null>(null)
  const clamp = (w: number) => Math.min(max, Math.max(min, w))

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    e.currentTarget.setPointerCapture(e.pointerId)
    e.currentTarget.dataset.dragging = 'true'
    drag.current = { x: e.clientX, width }
  }
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    const start = drag.current
    if (start) onWidth(clamp(start.width - (e.clientX - start.x)), false)
  }
  const onPointerUp = (e: PointerEvent<HTMLDivElement>) => {
    const start = drag.current
    if (!start) return
    drag.current = null
    delete e.currentTarget.dataset.dragging
    onWidth(clamp(start.width - (e.clientX - start.x)), true)
  }
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const next = { ArrowLeft: width + KEY_STEP, ArrowRight: width - KEY_STEP, Home: min, End: max }[e.key]
    if (next === undefined) return
    e.preventDefault()
    onWidth(clamp(next), true)
  }

  return (
    <div
      className="divider"
      role="separator"
      aria-orientation="vertical"
      aria-label="Resize file explorer"
      aria-valuemin={min}
      aria-valuemax={max}
      aria-valuenow={width}
      tabIndex={0}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onKeyDown={onKeyDown}
    />
  )
}
