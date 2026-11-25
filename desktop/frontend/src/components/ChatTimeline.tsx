import { useVirtualizer } from '@tanstack/react-virtual'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { followScroll, isAtBottom, keepScroll, keptScroll } from '../chatScroll'
import type { TimelineItem } from '../chatTimeline'
import { duration } from '../time'
import { ChatNotice } from './ChatNotice'
import { ChatReplyRow } from './ChatReplyRow'
import { ChatUserRow } from './ChatUserRow'
import { Icon } from './Icon'

type ChatTimelineProps = { id: string; items: TimelineItem[] }

// rowOf gives each row only the plain values it shows, so that a memoised row renders again only when they change:
// the timeline is built anew on every streamed delta, and only the streaming reply's text is new.
function rowOf(item: TimelineItem) {
  switch (item.kind) {
    case 'user':
      return <ChatUserRow text={item.text} />
    case 'reply':
      return <ChatReplyRow text={item.text} streaming={item.streaming} />
    case 'notice':
      return <ChatNotice text={item.text} error={item.error} />
    case 'work': {
      const calls = `${item.calls.length} tool ${item.calls.length === 1 ? 'call' : 'calls'}`
      return <ChatNotice text={item.endedAt === null ? `Working · ${calls}` : `Worked for ${duration(item.endedAt - item.startedAt)} · ${calls}`} error={false} />
    }
    case 'approval':
      return <ChatNotice text={item.expired ? `The approval to use ${item.tool} expired` : `Waiting for approval to use ${item.tool}`} error={false} />
    case 'question':
      return <ChatNotice text="Claude asked a question" error={false} />
    case 'plan':
      return <ChatNotice text="Claude proposed a plan" error={false} />
  }
}

export function ChatTimeline({ id, items }: ChatTimelineProps) {
  const scroller = useRef<HTMLDivElement>(null)
  const [kept] = useState(() => keptScroll(id))
  const [unseen, setUnseen] = useState(false)
  const following = useRef(kept === null)
  const lastTop = useRef(0)
  const lastCount = useRef(items.length)

  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => scroller.current,
    estimateSize: () => 120,
    overscan: 6,
    getItemKey: (index) => items[index].key,
    paddingStart: 24,
    paddingEnd: 8,
    initialOffset: kept?.top,
    initialMeasurementsCache: kept?.measured,
  })
  const total = virtualizer.getTotalSize()
  const lastTotal = useRef(total)

  // No run-once guard: when the virtualizer lets go of its scroll element and takes it back, as in a strict-mode
  // remount, it writes the offset it last read, so the view must be put back again.
  useLayoutEffect(() => {
    if (kept) virtualizer.scrollToOffset(kept.top)
    else virtualizer.scrollToEnd()
    return () => keepScroll(id, following.current ? null : { top: virtualizer.scrollOffset ?? 0, measured: virtualizer.takeSnapshot() })
  }, [id, kept, virtualizer])

  useLayoutEffect(() => {
    const grew = total > lastTotal.current
    lastTotal.current = total
    if (followScroll({ atBottom: following.current, grew }) === 'end') virtualizer.scrollToEnd()
  }, [total, virtualizer])

  useEffect(() => {
    if (items.length > lastCount.current && !following.current) setUnseen(true)
    lastCount.current = items.length
  }, [items.length])

  const onScroll = () => {
    const el = scroller.current
    if (!el) return
    // A row that grew before it was measured moves the end down with no scroll up, so only scrolling up stops the
    // follow.
    following.current = isAtBottom(el.scrollTop, el.clientHeight, el.scrollHeight) || (following.current && el.scrollTop >= lastTop.current)
    lastTop.current = el.scrollTop
    if (following.current) setUnseen(false)
  }

  const jump = () => {
    following.current = true
    setUnseen(false)
    virtualizer.scrollToEnd()
  }

  return (
    <div className="relative min-h-0 flex-1">
      <div ref={scroller} onScroll={onScroll} className="h-full overflow-y-auto">
        <div className="relative mx-auto w-full max-w-[46rem]" style={{ height: total }}>
          {virtualizer.getVirtualItems().map((row) => (
            <div key={row.key} data-index={row.index} ref={virtualizer.measureElement} className="absolute top-0 left-0 w-full px-4 pb-5" style={{ transform: `translateY(${row.start}px)` }}>
              {rowOf(items[row.index])}
            </div>
          ))}
        </div>
      </div>
      {unseen && (
        <button className="btn chat-jump absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full" onClick={jump}>
          <Icon name="arrow-down" size={12} />
          Jump to latest
        </button>
      )}
    </div>
  )
}
