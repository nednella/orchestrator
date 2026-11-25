import { ChevronDown, ChevronUp } from 'lucide-react'
import { memo, useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { KeyboardEvent, PointerEvent, RefObject, WheelEvent } from 'react'
import { isAtBottom } from '../chatScroll'
import { activeStrip, currentStrip, easeOutCubic, glideMs, hitStrip, inView, isNotched, minimapStrips, persistentGutter, preview, railHeight, stripScale, stripTop, wheelPixels } from '../chatMinimap'
import type { MinimapRow } from '../chatMinimap'
import type { TimelineItem } from '../chatTimeline'

// rows gives where each item sits in the scrolled content, rendered or not; onJump scrolls an item into view.
type ChatMinimapProps = { scroller: RefObject<HTMLDivElement>; lane: RefObject<HTMLDivElement>; items: TimelineItem[]; rows(): MinimapRow[]; total: number; onJump(index: number): void }

type Glide = { to: number; frame: number }

export const ChatMinimap = memo(function ChatMinimap({ scroller, lane, items, rows, total, onJump }: ChatMinimapProps) {
  const strips = useMemo(() => minimapStrips(items), [items])
  const latest = useRef(strips)
  latest.current = strips
  // The timeline is built anew on every streamed delta, so the strips are drawn again only when the list changes.
  const order = strips.map((strip) => strip.key).join(' ')
  const drawn = useMemo(() => strips, [order])
  const [pane, setPane] = useState({ height: 0, gutter: 0 })
  const [active, setActive] = useState<number | null>(null)
  const hit = useRef<HTMLButtonElement>(null)
  const next = useRef<HTMLButtonElement>(null)
  const nodes = useRef<(HTMLSpanElement | null)[]>([])
  const lit = useRef<boolean[]>([])
  const frame = useRef(0)
  const pressed = useRef(false)
  const glide = useRef<Glide | null>(null)
  const cardId = useId()

  // What is in view is written onto the strips straight, at most once a frame, so a scroll renders nothing.
  const refresh = useCallback(() => {
    if (frame.current) return
    frame.current = requestAnimationFrame(() => {
      frame.current = 0
      const el = scroller.current
      if (!el) return
      const shown = inView(latest.current, rows(), el.scrollTop, el.scrollTop + el.clientHeight)
      shown.forEach((on, i) => {
        const node = nodes.current[i]
        if (!node || lit.current[i] === on) return
        node.toggleAttribute('data-in-view', on)
        lit.current[i] = on
      })
      const end = isAtBottom(el.scrollTop, el.clientHeight, el.scrollHeight, 1)
      if (next.current && next.current.disabled !== end) next.current.disabled = end
    })
  }, [scroller, rows])

  useEffect(() => {
    const el = scroller.current
    const column = lane.current
    if (!el || !column) return
    const size = new ResizeObserver(() => {
      const height = el.clientHeight
      const gutter = column.getBoundingClientRect().left - el.getBoundingClientRect().left
      setPane((was) => (was.height === height && was.gutter === gutter ? was : { height, gutter }))
      refresh()
    })
    size.observe(el)
    size.observe(column)
    el.addEventListener('scroll', refresh, { passive: true })
    return () => {
      size.disconnect()
      el.removeEventListener('scroll', refresh)
      cancelAnimationFrame(frame.current)
      frame.current = 0
    }
  }, [scroller, lane, refresh])

  useLayoutEffect(() => {
    nodes.current.length = drawn.length
    lit.current = []
    refresh()
  }, [drawn, refresh])

  useEffect(refresh, [total, items, refresh])

  const stopGlide = useCallback(() => {
    if (glide.current) cancelAnimationFrame(glide.current.frame)
    glide.current = null
  }, [])

  useEffect(() => {
    window.addEventListener('pointerdown', stopGlide, { capture: true, passive: true })
    window.addEventListener('keydown', stopGlide, { capture: true, passive: true })
    return () => {
      window.removeEventListener('pointerdown', stopGlide, { capture: true })
      window.removeEventListener('keydown', stopGlide, { capture: true })
      stopGlide()
    }
  }, [stopGlide])

  const count = drawn.length
  const height = railHeight(count, pane.height)
  const strip = hitStrip(pane.gutter)

  const jump = (i: number) => {
    const target = latest.current[i]
    if (!target) return
    stopGlide()
    onJump(target.index)
  }

  const current = () => {
    const el = scroller.current
    return el ? currentStrip(latest.current, rows(), el.scrollTop, el.scrollTop + el.clientHeight) : -1
  }

  const indexAt = (clientY: number) => {
    const rail = hit.current?.getBoundingClientRect()
    return rail ? activeStrip(clientY - rail.top, rail.height, count) : 0
  }

  const onPointerDown = (e: PointerEvent<HTMLButtonElement>) => {
    if (e.button !== 0) return
    pressed.current = true
    e.currentTarget.setPointerCapture(e.pointerId)
    setActive(indexAt(e.clientY))
  }

  // A press that began outside, as a text selection that runs into the gutter, is not the minimap's.
  const onPointerMove = (e: PointerEvent<HTMLButtonElement>) => {
    if (!pressed.current && e.buttons !== 0) return
    setActive(indexAt(e.clientY))
  }

  const onPointerUp = (e: PointerEvent<HTMLButtonElement>) => {
    if (!pressed.current) return
    pressed.current = false
    jump(indexAt(e.clientY))
    setActive(null)
    e.currentTarget.blur()
  }

  const onPointerCancel = () => {
    pressed.current = false
    setActive(null)
  }

  const onPointerLeave = () => {
    if (!pressed.current) setActive(null)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLButtonElement>) => {
    const at = active ?? 0
    const moves: Record<string, number> = { ArrowUp: at - 1, ArrowDown: at + 1, Home: 0, End: count - 1 }
    if (e.key in moves) setActive(Math.min(Math.max(moves[e.key], 0), count - 1))
    else if (e.key === 'Enter' || e.key === ' ') jump(at)
    else return
    e.preventDefault()
    e.stopPropagation()
  }

  const step = (by: -1 | 1) => {
    const to = current() + by
    if (to >= 0 && to < count) jump(to)
  }

  // A trackpad scrolls the chat by its delta at once; a wheel that turns in notches glides, and a notch during a glide
  // adds to what is left of it.
  const onWheel = (e: WheelEvent<HTMLDivElement>) => {
    const el = scroller.current
    if (!el) return
    const delta = wheelPixels(e.deltaY, e.deltaMode, el.clientHeight)
    const legacy = (e.nativeEvent as globalThis.WheelEvent & { wheelDeltaY?: number }).wheelDeltaY ?? 0
    if (!isNotched(e.deltaMode, legacy)) {
      stopGlide()
      el.scrollTop += delta
      return
    }
    const max = el.scrollHeight - el.clientHeight
    const to = Math.min(Math.max((glide.current?.to ?? el.scrollTop) + delta, 0), max)
    stopGlide()
    const from = el.scrollTop
    const started = performance.now()
    const move = (now: number) => {
      // A frame's time can be from just before the glide began.
      const t = Math.min(Math.max((now - started) / glideMs, 0), 1)
      el.scrollTop = from + (to - from) * easeOutCubic(t)
      if (t < 1 && glide.current) glide.current.frame = requestAnimationFrame(move)
      else glide.current = null
    }
    glide.current = { to, frame: requestAnimationFrame(move) }
  }

  const marks = useMemo(
    () =>
      drawn.map((s, i) => (
        <span
          key={s.key}
          ref={(node) => {
            nodes.current[i] = node
          }}
          data-kind={s.kind}
          data-active={active === i ? '' : undefined}
          className="chat-minimap-strip"
          style={{ top: stripTop(i, count, height), transform: `scaleX(${stripScale(active === null ? null : Math.abs(i - active))})` }}
        />
      )),
    [drawn, active, count, height],
  )

  if (count < 2) return null

  const at = Math.min(active ?? 0, count - 1)
  const spoken = preview(strips[at])
  const card = active === null ? null : spoken
  const reachable = strip.width >= 14

  return (
    <div className="chat-minimap" data-persistent={persistentGutter(pane.gutter) ? '' : undefined}>
      <div className="chat-minimap-rail" style={{ left: strip.left, width: strip.width, height }} onWheel={onWheel}>
        <button type="button" aria-label="Previous prompt or error" data-reachable={reachable ? '' : undefined} tabIndex={reachable ? undefined : -1} className="chat-minimap-step" style={{ bottom: 'calc(100% + 6px)' }} onClick={() => step(-1)}>
          <ChevronUp size={16} />
        </button>
        <button
          ref={hit}
          type="button"
          role="slider"
          aria-label="Prompts and errors in this thread"
          aria-orientation="vertical"
          aria-valuemin={1}
          aria-valuemax={count}
          aria-valuenow={at + 1}
          aria-valuetext={strips[at].kind === 'error' ? `Error: ${spoken.title}` : spoken.title}
          aria-describedby={card ? cardId : undefined}
          className="chat-minimap-hit"
          onMouseDown={(e) => e.preventDefault()}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={onPointerCancel}
          onPointerLeave={onPointerLeave}
          onFocus={() => setActive(Math.max(current(), 0))}
          onBlur={() => setActive(null)}
          onKeyDown={onKeyDown}
        >
          {marks}
        </button>
        <button ref={next} type="button" aria-label="Next prompt or error" data-reachable={reachable ? '' : undefined} tabIndex={reachable ? undefined : -1} className="chat-minimap-step" style={{ top: 'calc(100% + 6px)' }} onClick={() => step(1)}>
          <ChevronDown size={16} />
        </button>
        {card && (
          <div id={cardId} className="chat-minimap-card" style={{ top: stripTop(at, count, height) }}>
            <p className="chat-minimap-card-title">{card.title}</p>
            {card.detail && <p className="chat-minimap-card-detail">{card.detail}</p>}
          </div>
        )}
      </div>
    </div>
  )
})
