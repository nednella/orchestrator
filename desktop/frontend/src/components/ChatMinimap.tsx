import { memo, useEffect, useMemo, useRef, useState, type PointerEvent, type RefObject } from 'react'
import { minimapMarks, thumb, trackToScroll, type MinimapMark } from '../chatScroll'
import type { TimelineItem } from '../chatTimeline'

type ChatMinimapProps = { scroller: RefObject<HTMLDivElement>; items: TimelineItem[]; starts: number[]; total: number }

const markColor: Record<MinimapMark['kind'], string> = { user: 'var(--text-dim)', approval: 'var(--waiting)', error: 'var(--danger)' }

const firstLine = (s: string) => s.split('\n').find((l) => l.trim() !== '')?.trim() ?? ''

function markTitle(item: TimelineItem | undefined): string {
  if (item?.kind === 'user' || item?.kind === 'notice') return firstLine(item.text)
  if (item?.kind === 'approval') return `Approval: ${item.tool}`
  return ''
}

export const ChatMinimap = memo(function ChatMinimap({ scroller, items, starts, total }: ChatMinimapProps) {
  const track = useRef<HTMLDivElement>(null)
  const [height, setHeight] = useState(0)
  const [view, setView] = useState({ top: 0, client: 0 })
  const drag = useRef<{ y: number; top: number } | null>(null)

  useEffect(() => {
    const el = scroller.current
    const bar = track.current
    if (!el || !bar) return
    const read = () => setView({ top: el.scrollTop, client: el.clientHeight })
    const size = new ResizeObserver(() => {
      setHeight(bar.clientHeight)
      read()
    })
    size.observe(bar)
    el.addEventListener('scroll', read, { passive: true })
    read()
    return () => {
      size.disconnect()
      el.removeEventListener('scroll', read)
    }
  }, [scroller])

  const marks = useMemo(() => {
    const byKey = new Map(items.map((item) => [item.key, item]))
    return minimapMarks(items, starts, total, height).map((mark) => ({ ...mark, title: markTitle(byKey.get(mark.key)) }))
  }, [items, starts, total, height])

  const handle = thumb(view.top, view.client, total, height)
  const hidden = total <= view.client

  const jump = (e: PointerEvent<HTMLDivElement>) => {
    const el = scroller.current
    if (!el) return
    const y = e.clientY - e.currentTarget.getBoundingClientRect().top
    el.scrollTop = trackToScroll(y, height, total, el.clientHeight)
  }

  const startDrag = (e: PointerEvent<HTMLDivElement>) => {
    e.stopPropagation()
    e.currentTarget.setPointerCapture(e.pointerId)
    drag.current = { y: e.clientY, top: scroller.current?.scrollTop ?? 0 }
  }

  const moveDrag = (e: PointerEvent<HTMLDivElement>) => {
    const el = scroller.current
    if (!drag.current || !el) return
    const room = height - handle.size
    if (room <= 0) return
    el.scrollTop = drag.current.top + ((e.clientY - drag.current.y) * (total - el.clientHeight)) / room
  }

  return (
    <div ref={track} onPointerDown={jump} aria-hidden className={`absolute top-0 right-0 bottom-0 w-[10px] select-none ${hidden ? 'hidden' : ''}`}>
      <div
        className="absolute inset-x-0 rounded-sm bg-[var(--bg-hover)] ring-1 ring-[var(--border-strong)]"
        style={{ top: handle.top, height: handle.size }}
        onPointerDown={startDrag}
        onPointerMove={moveDrag}
        onPointerUp={() => (drag.current = null)}
        onPointerCancel={() => (drag.current = null)}
      />
      {marks.map((mark) => (
        <div key={mark.key} title={mark.title} className="absolute right-0 h-[3px] w-1.5 rounded-sm" style={{ top: mark.top, background: markColor[mark.kind] }} />
      ))}
    </div>
  )
})
