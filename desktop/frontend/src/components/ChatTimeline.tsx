import { useVirtualizer } from '@tanstack/react-virtual'
import { useCallback, useEffect, useImperativeHandle, useLayoutEffect, useRef, useState } from 'react'
import type { Ref } from 'react'
import { ACCESS_LABEL } from '../access'
import { useOrchestrator } from '../OrchestratorContext'
import { useShowAgents } from '../chatAgents'
import { jumpOffset } from '../chatMinimap'
import { ANCHOR_SLACK, followScroll, keepScroll, keptScroll, stillFollows } from '../chatScroll'
import type { ChildItem, TimelineItem } from '../chatTimeline'
import { ChatAgentRow } from './ChatAgentRow'
import { ChatApprovalCard } from './ChatApprovalCard'
import { ChatChildCard } from './ChatChildCard'
import { ChatForkDivider } from './ChatForkDivider'
import { ChatLineageDivider } from './ChatLineageDivider'
import { ChatMessageActions } from './ChatMessageActions'
import { ChatMinimap } from './ChatMinimap'
import { ChatNotice } from './ChatNotice'
import { ChatPlanCard } from './ChatPlanCard'
import { ChatQuestionCard } from './ChatQuestionCard'
import { ChatQuoteButton } from './ChatQuoteButton'
import { ChatReplyRow } from './ChatReplyRow'
import { ChatThinkingRow } from './ChatThinkingRow'
import { ChatUserRow } from './ChatUserRow'
import { ChatWorkingRow } from './ChatWorkingRow'
import { ChatWorkRow } from './ChatWorkRow'
import { ImageViewer } from './ImageViewer'

export type ChatTimelineEnd = { scrollToEnd(): void }

// ChatEdit is Edit from here on the owner's messages: why it is off for the message with a uuid, '' when it is on, and
// a way to ask for it, which stays the same from render to render so that the rows stay memoised.
export type ChatEdit = { blocked(uuid: string): string; onOpen(uuid: string): void }

// inset is the height at the bottom that the composer covers, which the last row scrolls clear of. onFollowing says
// whether the chat follows its end, as the reader scrolls away from it and back. onFork is left out where no reply
// forks, as in a side chat.
type ChatTimelineProps = {
  id: string
  items: TimelineItem[]
  folder: string
  inset: number
  endRef: Ref<ChatTimelineEnd>
  onFollowing(following: boolean): void
  onReply(text: string): void
  onQuote(text: string): void
  onFork?(at: string, draft: string): Promise<void>
  edit?: ChatEdit
}

// rowOf gives each row only the plain values it shows, so that a memoised row renders again only when they change:
// the timeline is built anew on every streamed delta, and only the streaming reply's text is new.
function rowOf(items: TimelineItem[], index: number, thread: string, folder: string, onReply: (text: string) => void, onFork: ((at: string, draft: string) => Promise<void>) | undefined, onOpenImage: (url: string) => void, onOpenThread: (id: string) => void, edit?: ChatEdit) {
  const item = items[index]
  switch (item.kind) {
    case 'user':
      return (
        <ChatUserRow
          thread={thread}
          id={item.id}
          uuid={item.uuid}
          text={item.text}
          files={item.files}
          from={item.from}
          at={item.at}
          afterUser={items[index - 1]?.kind === 'user'}
          beforeUser={items[index + 1]?.kind === 'user'}
          // A message another thread sent is not the owner's to edit.
          editBlocked={item.from ? undefined : edit?.blocked(item.uuid)}
          onEditFrom={item.from ? undefined : edit?.onOpen}
          onOpenImage={onOpenImage}
        />
      )
    case 'reply': {
      const next = items[index + 1]
      return <ChatReplyRow thread={thread} uuid={item.uuid} text={item.text} at={item.at} streaming={item.streaming} ended={item.ended} toolsAfter={next?.kind === 'work' && next.endedAt === null} onReply={onReply} onFork={onFork} />
    }
    case 'notice':
      return <ChatNotice text={item.text} error={item.error} />
    case 'restart': {
      // The access comes from the log as text, so a value this build does not know has no label.
      const label: string | undefined = ACCESS_LABEL[item.access]
      return <ChatNotice text={label ? `Restarted with ${label}` : 'Restarted'} error={false} />
    }
    case 'work': {
      const work = <ChatWorkRow thread={thread} turn={item.key} steps={item.steps} startedAt={item.startedAt} endedAt={item.endedAt} folder={folder} />
      // The reply these tool rows follow shows its actions under them. A turn's fold follows no reply of its own turn.
      const reply = items[index - 1]
      if (reply?.kind !== 'reply' || item.endedAt !== null) return work
      return (
        <>
          {work}
          <ChatMessageActions text={reply.text} at={reply.at} streaming={reply.streaming} ended={reply.ended} shown onReply={onReply} onFork={reply.uuid && onFork ? () => onFork(reply.uuid, '') : undefined} />
        </>
      )
    }
    case 'working':
      return <ChatWorkingRow startedAt={item.startedAt} />
    case 'thinking':
      return <ChatThinkingRow label={item.label} />
    case 'approval':
      return <ChatApprovalCard thread={thread} request={item.request} tool={item.tool} input={item.input} expired={item.expired} folder={folder} />
    case 'question':
      return <ChatQuestionCard thread={thread} id={item.id} request={item.request} input={item.input} answer={item.answer} answers={item.answers} expired={item.expired} result={item.result} />
    case 'plan':
      return <ChatPlanCard thread={thread} request={item.request} input={item.input} answer={item.answer} expired={item.expired} result={item.result} />
    case 'agent':
      return <ChatAgentRow thread={thread} id={item.id} description={item.description} type={item.type} state={item.state} startedAt={item.startedAt} endedAt={item.endedAt} tools={item.tools} steps={item.steps} result={item.result} folder={folder} />
    case 'child':
      if (item.error) return <ChatNotice text={`${item.title} did not start`} error />
      if (item.expired) return <ChatNotice text={`${item.title}: the call ended before it said which thread it started`} error={false} />
      return <ChatChildCard id={item.child} title={item.title} />
    case 'forkedFrom':
      return <ChatForkDivider thread={item.source} label="Forked from conversation" hint="Open source conversation" />
    case 'forked':
      return <ChatForkDivider thread={item.fork} label={`Conversation forked · ${item.title}`} hint="Open fork" />
    case 'lineage':
      return <ChatLineageDivider parent={item.title} onOpenParent={() => onOpenThread(item.parent)} />
    case 'childResult': {
      const title = items.find((i): i is ChildItem => i.kind === 'child' && i.child === item.child)?.title ?? 'Child thread'
      return <ChatChildCard id={item.child} title={title} result={item.text} />
    }
  }
}

// headsSteps is whether the row is the working row with the turn's first steps right under it, which sit at the rule's
// own space below it.
const headsSteps = (items: TimelineItem[], index: number) => items[index].kind === 'working' && items[index + 1]?.kind === 'work'

export function ChatTimeline({ id, items, folder, inset, endRef, onFollowing, onReply, onQuote, onFork, edit }: ChatTimelineProps) {
  const { select } = useOrchestrator('select')
  const scroller = useRef<HTMLDivElement>(null)
  const lane = useRef<HTMLDivElement>(null)
  const [kept] = useState(() => keptScroll(id))
  const following = useRef(kept === null)
  const lastTop = useRef(0)
  // A row is moved into place with a transform, which would hold a fixed overlay inside it.
  const [viewing, setViewing] = useState<string | null>(null)

  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => scroller.current,
    estimateSize: () => 120,
    overscan: 6,
    getItemKey: (index) => items[index].key,
    paddingStart: 24,
    paddingEnd: inset,
    scrollPaddingStart: jumpOffset,
    initialOffset: kept?.top,
    initialMeasurementsCache: kept?.measured,
    // Near the end, a row that grows moves the view with it before the frame is painted, and a row added scrolls the
    // view to the end.
    anchorTo: 'end',
    scrollEndThreshold: ANCHOR_SLACK,
    followOnAppend: true,
  })
  const total = virtualizer.getTotalSize()
  const last = useRef({ total, inset })
  // The virtualizer's measurements cover every row, rendered or not; a row not yet measured counts at its estimate.
  const rows = useCallback(() => virtualizer.measurementsCache, [virtualizer])
  const scrollToItem = useCallback((index: number) => virtualizer.scrollToIndex(index, { align: 'start', behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' }), [virtualizer])
  useShowAgents(id, items)

  // No run-once guard: when the virtualizer lets go of its scroll element and takes it back, as in a strict-mode
  // remount, it writes the offset it last read, so the view must be put back again.
  useLayoutEffect(() => {
    if (kept) virtualizer.scrollToOffset(kept.top)
    else virtualizer.scrollToEnd()
    return () => keepScroll(id, following.current ? null : { top: virtualizer.scrollOffset ?? 0, measured: virtualizer.takeSnapshot() })
  }, [id, kept, virtualizer])

  useLayoutEffect(() => {
    const el = scroller.current
    const grew = total > last.current.total
    const insetGrew = inset > last.current.inset
    last.current = { total, inset }
    if (!el || !(grew || insetGrew)) return
    const fromEnd = el.scrollHeight - el.scrollTop - el.clientHeight
    if (followScroll({ following: following.current, grew, insetGrew, fromEnd }) === 'end') virtualizer.scrollToEnd()
  }, [total, inset, virtualizer])

  useEffect(() => onFollowing(following.current), [onFollowing])

  useEffect(() => {
    const el = scroller.current
    // The thread took the focus while its chat was still loading.
    if (el && document.activeElement === el.closest('[data-panel]')) el.focus()
  }, [])

  const onScroll = () => {
    const el = scroller.current
    if (!el) return
    const next = stillFollows({ following: following.current, top: el.scrollTop, lastTop: lastTop.current, client: el.clientHeight, height: el.scrollHeight })
    lastTop.current = el.scrollTop
    if (next === following.current) return
    following.current = next
    onFollowing(next)
  }

  useImperativeHandle(
    endRef,
    () => ({
      scrollToEnd: () => {
        following.current = true
        onFollowing(true)
        virtualizer.scrollToEnd({ behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' })
      },
    }),
    [virtualizer, onFollowing],
  )

  return (
    <div className="relative min-h-0 flex-1">
      <div ref={scroller} onScroll={onScroll} data-chat-scroll tabIndex={-1} className="chat-scroll h-full overflow-y-auto outline-none">
        <div ref={lane} className="chat-column relative" style={{ height: total }}>
          {virtualizer.getVirtualItems().map((row) => (
            <div key={row.key} data-index={row.index} ref={virtualizer.measureElement} className={`absolute top-0 left-0 w-full ${headsSteps(items, row.index) ? '' : 'pb-5'}`} style={{ transform: `translateY(${row.start}px)` }}>
              {rowOf(items, row.index, id, folder, onReply, onFork, setViewing, select, edit)}
            </div>
          ))}
        </div>
      </div>
      <ChatMinimap scroller={scroller} lane={lane} items={items} rows={rows} total={total} onJump={scrollToItem} />
      <ChatQuoteButton onQuote={onQuote} />
      {viewing && <ImageViewer url={viewing} alt="Attached to the message" onClose={() => setViewing(null)} />}
    </div>
  )
}
