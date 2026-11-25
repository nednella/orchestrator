import { useEffect, useRef } from 'react'
import type { KeyboardEvent, PointerEvent, RefObject } from 'react'
import { COLUMN_DEFAULT_REM, clampColumn, dragColumn, rootRemPx } from '../chatColumn'

// A press that moves less than this is a click, so a double-click resets the width without first moving it.
const MOVE_PX = 2
const SETTLE_MS = 200
const KEY_STEP_REM = 1

type Drag = { pointer: number; x: number; start: number; remPx: number; fullRem: number; moved: boolean; width: number; frame: number; blur(): void }

// side is the column edge the handle sits on. The column is centred, so it grows by twice what the edge moves. pane is
// the chat pane: it holds the width while a drag shows it, and its width is the column's full width.
type ChatColumnHandleProps = { side: 'left' | 'right'; pane: RefObject<HTMLElement | null>; column: number; onColumn(rem: number): void }

export function ChatColumnHandle({ side, pane, column, onColumn }: ChatColumnHandleProps) {
  const drag = useRef<Drag | null>(null)
  const settling = useRef(0)
  const outward = side === 'right' ? 1 : -1

  useEffect(
    () => () => {
      clearTimeout(settling.current)
      const d = drag.current
      if (!d) return
      cancelAnimationFrame(d.frame)
      window.removeEventListener('blur', d.blur)
      delete document.documentElement.dataset.columnDrag
    },
    [],
  )

  const fullRem = (remPx: number) => (pane.current?.clientWidth ?? 0) / remPx
  const show = (rem: number) => pane.current?.style.setProperty('--chat-column', `${rem}rem`)

  // settle eases the column to its new width and keeps it.
  const settle = (rem: number) => {
    const el = pane.current
    if (!el) return
    clearTimeout(settling.current)
    el.dataset.columnSettling = 'true'
    show(rem)
    settling.current = window.setTimeout(() => delete el.dataset.columnSettling, SETTLE_MS)
    onColumn(rem)
  }

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return
    e.preventDefault()
    const handle = e.currentTarget
    handle.setPointerCapture(e.pointerId)
    handle.dataset.dragging = 'true'
    document.documentElement.dataset.columnDrag = 'true'
    const remPx = rootRemPx()
    const full = fullRem(remPx)
    const start = clampColumn(column, full)
    const pointer = e.pointerId
    const blur = () => handle.releasePointerCapture(pointer)
    window.addEventListener('blur', blur)
    drag.current = { pointer, x: e.clientX, start, remPx, fullRem: full, moved: false, width: start, frame: 0, blur }
  }

  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    const d = drag.current
    if (!d || e.pointerId !== d.pointer) return
    const dx = e.clientX - d.x
    if (!d.moved && Math.abs(dx) <= MOVE_PX) return
    d.moved = true
    d.width = dragColumn(d.start + (2 * outward * dx) / d.remPx, d.fullRem)
    if (d.frame) return
    d.frame = requestAnimationFrame(() => {
      d.frame = 0
      show(d.width)
    })
  }

  // The capture goes when the button is let go, the pointer is cancelled or the window loses focus: each ends the drag.
  const onLostPointerCapture = (e: PointerEvent<HTMLDivElement>) => {
    const d = drag.current
    if (!d) return
    drag.current = null
    cancelAnimationFrame(d.frame)
    window.removeEventListener('blur', d.blur)
    delete e.currentTarget.dataset.dragging
    delete document.documentElement.dataset.columnDrag
    if (d.moved) settle(clampColumn(d.width, d.fullRem))
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    const full = fullRem(rootRemPx())
    const grow = (e.key === 'ArrowRight' ? 1 : -1) * outward
    settle(clampColumn(clampColumn(column, full) + 2 * grow * KEY_STEP_REM, full))
  }

  return (
    <div
      className="column-handle"
      data-side={side}
      role="separator"
      aria-orientation="vertical"
      aria-label="Message column width"
      aria-valuemin={COLUMN_DEFAULT_REM}
      aria-valuenow={column}
      tabIndex={0}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onLostPointerCapture={onLostPointerCapture}
      onDoubleClick={() => settle(COLUMN_DEFAULT_REM)}
      onKeyDown={onKeyDown}
    />
  )
}
