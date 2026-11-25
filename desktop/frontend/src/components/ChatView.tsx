import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos, useLatestAgentos } from '../AgentosContext'
import { api } from '../api'
import { escapeAction, lastTurnEnd, quoteMarkdown } from '../chatComposer'
import type { EscapeAction } from '../chatComposer'
import { fromMarkdown } from '../composerMarkdown'
import type { Session } from '../types'
import { useChat } from '../useChat'
import { isTyping } from '../useListNav'
import { ChatEndPill } from './ChatEndPill'
import { ChatNotice } from './ChatNotice'
import { ChatTimeline } from './ChatTimeline'
import type { ChatTimelineEnd } from './ChatTimeline'
import type { ComposerInput } from './ComposerEditor'
import { Loading } from './Loading'
import { Notice } from './Notice'
import { ThreadComposer } from './ThreadComposer'

// A stop that gets no turn end in this long can be asked for again.
const stopWait = 10_000
// The gap between the chat's last row, scrolled to the end, and the composer over it.
const insetGap = 12

type ChatViewProps = { session: Session }

export function ChatView({ session }: ChatViewProps) {
  const { project, report, forkSession } = useAgentos('project', 'report', 'forkSession')
  const latest = useLatestAgentos()
  const { timeline, events, loading, error } = useChat(session.id, session.state)
  const empty = timeline.items.length === 0
  const lastEnd = lastTurnEnd(events)
  // The last turn end before the stop was asked for: the stop holds until a later one.
  const [stopAsked, setStopAsked] = useState<number | null>(null)
  const stopping = timeline.turnOpen && stopAsked === lastEnd

  useEffect(() => {
    if (!stopping) return
    const timer = setTimeout(() => setStopAsked(null), stopWait)
    return () => clearTimeout(timer)
  }, [stopping])

  const stopWith = (call: () => Promise<void>) => {
    if (stopping) return
    setStopAsked(lastEnd)
    report(() =>
      call().catch((err) => {
        setStopAsked(null)
        throw err
      }),
    )
  }
  const input = useRef<ComposerInput>(null)
  const end = useRef<ChatTimelineEnd>(null)
  const composer = useRef<HTMLDivElement>(null)
  const [inset, setInset] = useState(0)
  const [following, setFollowing] = useState(true)

  useLayoutEffect(() => {
    const el = composer.current
    if (!el) return
    const measure = () => setInset(el.offsetHeight + insetGap)
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  // The message the next send answers.
  const [reply, setReply] = useState<string | null>(null)
  const replyTo = useCallback((text: string) => {
    setReply(text)
    input.current?.focus()
  }, [])
  const quote = useCallback((selection: string) => {
    const quoted = fromMarkdown(quoteMarkdown(selection))
    // The empty paragraph after the quote puts the caret outside it.
    input.current?.insertDoc({ ...quoted, content: [...(quoted.content ?? []), { type: 'paragraph' }] })
    input.current?.focus()
  }, [])

  // The promise settles once the fork is made or has failed, which the toast reports.
  const fork = useCallback(
    (at: string, draft: string) => {
      const forked = forkSession(session.id, at, draft)
      report(() => forked)
      return forked.then(
        () => undefined,
        () => undefined,
      )
    },
    [session.id, report, forkSession],
  )

  const stop = () => stopWith(() => api.chatInterrupt(session.id))
  const sendNow = (message: string) => stopWith(() => api.chatSendNow(session.id, message))

  // Esc never sends what is typed in the composer: it stays there.
  const escape = (picker: boolean): EscapeAction => {
    const queued = timeline.queued[0]
    const action = escapeAction({ overlay: latest().overlay !== null, picker, turnOpen: timeline.turnOpen, stopping, queued: queued !== undefined })
    if (action === 'send-now') sendNow(queued.id)
    if (action === 'interrupt') stop()
    return action
  }

  // A text field, the composer among them, takes its own Esc.
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== 'Escape' || isTyping(e.target)) return
    const action = escape(false)
    if (action === 'send-now' || action === 'interrupt') e.preventDefault()
  }

  return (
    <div data-chat className="relative flex h-full min-h-0 flex-col bg-app" onKeyDown={onKeyDown}>
      {empty && loading && <Loading />}
      {empty && !loading && error && !session.hosted && <p className="flex flex-1 items-center justify-center px-4 text-center text-small text-dim">{error}</p>}
      {empty && !loading && error && session.hosted && <Notice title="The chat did not load" hint={error} centered />}
      {empty && !loading && !error && <div className="flex-1" />}
      {!empty && (
        <ChatTimeline
          id={session.id}
          items={timeline.items}
          folder={session.worktree || project?.dir || ''}
          inset={inset}
          endRef={end}
          onFollowing={setFollowing}
          onReply={replyTo}
          onQuote={quote}
          onFork={fork}
        />
      )}
      {/* The chat runs on under the composer; only the composer itself takes the pointer. */}
      <div ref={composer} className="pointer-events-none absolute inset-x-0 bottom-0 z-20 pt-1.5 pb-4 sm:pt-2 sm:pb-5">
        <div className="mx-auto w-full max-w-[46rem] px-4">
          <div className="pointer-events-auto relative">
            {!empty && <ChatEndPill shown={!following} queued={timeline.queued.length} onClick={() => end.current?.scrollToEnd()} />}
            {!empty && error && (
              <div className="glass mx-auto mb-2 w-fit max-w-full rounded-lg px-2 py-0.5">
                <ChatNotice text={error} error />
              </div>
            )}
            <ThreadComposer inputRef={input} reply={reply} onReply={setReply} session={session} timeline={timeline} events={events} stopping={stopping} onStop={stop} onSendNow={sendNow} onEscape={escape} />
          </div>
        </div>
      </div>
    </div>
  )
}
