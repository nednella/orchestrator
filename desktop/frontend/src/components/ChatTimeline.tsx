import { useVirtualizer } from '@tanstack/react-virtual'
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { followScroll, hasUnseen, isAtBottom, keepScroll, keptScroll } from '../chatScroll'
import type { TimelineItem } from '../chatTimeline'
import { ChatApprovalCard } from './ChatApprovalCard'
import { ChatMinimap } from './ChatMinimap'
import { ChatNotice } from './ChatNotice'
import { ChatPlanCard } from './ChatPlanCard'
import { ChatQuestionCard } from './ChatQuestionCard'
import { ChatQuoteButton } from './ChatQuoteButton'
import { ChatReplyRow } from './ChatReplyRow'
import { ChatUserRow } from './ChatUserRow'
import { ChatWorkRow } from './ChatWorkRow'
import { Icon } from './Icon'
import { ImageViewer } from './ImageViewer'

type ChatTimelineProps = { id: string; items: TimelineItem[]; folder: string; onReply(text: string): void; onQuote(text: string): void; onFork(at: string, draft: string): void }

// rowOf gives each row only the plain values it shows, so that a memoised row renders again only when they change:
// the timeline is built anew on every streamed delta, and only the streaming reply's text is new.
function rowOf(item: TimelineItem, thread: string, folder: string, onReply: (text: string) => void, onFork: (at: string, draft: string) => void, onOpenImage: (url: string) => void) {
  switch (item.kind) {
    case 'user':
      return <ChatUserRow thread={thread} text={item.text} files={item.files} forkAt={item.forkAt} onReply={onReply} onFork={onFork} onOpenImage={onOpenImage} />
    case 'reply':
      return <ChatReplyRow uuid={item.uuid} text={item.text} streaming={item.streaming} onReply={onReply} onFork={onFork} />
    case 'notice':
      return <ChatNotice text={item.text} error={item.error} />
    case 'work':
      return <ChatWorkRow thread={thread} turn={item.key} steps={item.steps} startedAt={item.startedAt} endedAt={item.endedAt} folder={folder} />
    case 'approval':
      return <ChatApprovalCard thread={thread} request={item.request} tool={item.tool} input={item.input} expired={item.expired} folder={folder} />
    case 'question':
      return <ChatQuestionCard thread={thread} id={item.id} request={item.request} input={item.input} answer={item.answer} answers={item.answers} expired={item.expired} result={item.result} />
    case 'plan':
      return <ChatPlanCard thread={thread} request={item.request} input={item.input} answer={item.answer} expired={item.expired} result={item.result} />
  }
}

export function ChatTimeline({ id, items, folder, onReply, onQuote, onFork }: ChatTimelineProps) {
  const scroller = useRef<HTMLDivElement>(null)
  const [kept] = useState(() => keptScroll(id))
  const [unseen, setUnseen] = useState(() => hasUnseen(kept, items.length))
  const following = useRef(kept === null)
  const lastTop = useRef(0)
  const seen = useRef(kept?.count ?? items.length)
  // A row is moved into place with a transform, which would hold a fixed overlay inside it.
  const [viewing, setViewing] = useState<string | null>(null)

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
  // The virtualizer's measurements cover every row, rendered or not; a row not yet measured counts at its estimate.
  const starts = useMemo(() => virtualizer.measurementsCache.map((m) => m.start), [virtualizer, total, items.length])

  // No run-once guard: when the virtualizer lets go of its scroll element and takes it back, as in a strict-mode
  // remount, it writes the offset it last read, so the view must be put back again.
  useLayoutEffect(() => {
    if (kept) virtualizer.scrollToOffset(kept.top)
    else virtualizer.scrollToEnd()
    return () => keepScroll(id, following.current ? null : { top: virtualizer.scrollOffset ?? 0, measured: virtualizer.takeSnapshot(), count: seen.current })
  }, [id, kept, virtualizer])

  useLayoutEffect(() => {
    const grew = total > lastTotal.current
    lastTotal.current = total
    if (followScroll({ atBottom: following.current, grew }) === 'end') virtualizer.scrollToEnd()
  }, [total, virtualizer])

  useEffect(() => {
    if (following.current) seen.current = items.length
    else if (items.length > seen.current) setUnseen(true)
  }, [items.length])

  useEffect(() => {
    const el = scroller.current
    // The thread took the focus while its chat was still loading.
    if (el && document.activeElement === el.closest('[data-panel]')) el.focus()
  }, [])

  const onScroll = () => {
    const el = scroller.current
    if (!el) return
    // A row that grew before it was measured moves the end down with no scroll up, so only scrolling up stops the
    // follow.
    following.current = isAtBottom(el.scrollTop, el.clientHeight, el.scrollHeight) || (following.current && el.scrollTop >= lastTop.current)
    lastTop.current = el.scrollTop
    if (!following.current) return
    seen.current = items.length
    setUnseen(false)
  }

  const jump = () => {
    following.current = true
    seen.current = items.length
    setUnseen(false)
    virtualizer.scrollToEnd()
  }

  return (
    <div className="relative min-h-0 flex-1">
      <div ref={scroller} onScroll={onScroll} data-chat-scroll tabIndex={-1} className="chat-scroll h-full overflow-y-auto outline-none">
        <div className="relative mx-auto w-full max-w-[46rem]" style={{ height: total }}>
          {virtualizer.getVirtualItems().map((row) => (
            <div key={row.key} data-index={row.index} ref={virtualizer.measureElement} className="absolute top-0 left-0 w-full px-4 pb-5" style={{ transform: `translateY(${row.start}px)` }}>
              {rowOf(items[row.index], id, folder, onReply, onFork, setViewing)}
            </div>
          ))}
        </div>
      </div>
      <ChatMinimap scroller={scroller} items={items} starts={starts} total={total} />
      <ChatQuoteButton onQuote={onQuote} />
      {viewing && <ImageViewer url={viewing} alt="Attached to the message" onClose={() => setViewing(null)} />}
      {unseen && (
        <button className="btn chat-jump absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full" onClick={jump}>
          <Icon name="arrow-down" size={12} />
          Jump to latest
        </button>
      )}
    </div>
  )
}
