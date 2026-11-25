import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { CSSProperties, KeyboardEvent } from 'react'
import { useAgentos, useLatestAgentos } from '../AgentosContext'
import { api } from '../api'
import { COLUMN_DEFAULT_REM, COLUMN_KEY, rootRemPx, storedColumn } from '../chatColumn'
import { escapeAction, lastTurnEnd, quoteMarkdown } from '../chatComposer'
import type { EscapeAction } from '../chatComposer'
import { editBlocked } from '../chatEdit'
import { fromMarkdown } from '../composerMarkdown'
import { readStored, writeStored } from '../storage'
import type { Session } from '../types'
import { useChat } from '../useChat'
import { isTyping } from '../useListNav'
import { ChatColumnHandle } from './ChatColumnHandle'
import { ChatEditConfirm } from './ChatEditConfirm'
import { ChatEndPill } from './ChatEndPill'
import { ChatNotice } from './ChatNotice'
import { ChatTimeline } from './ChatTimeline'
import type { ChatEdit, ChatTimelineEnd } from './ChatTimeline'
import type { ComposerInput } from './ComposerEditor'
import { Loading } from './Loading'
import { Notice } from './Notice'
import { ThreadComposer } from './ThreadComposer'
import type { ThreadComposerDraft } from './ThreadComposer'

// A stop that gets no turn end in this long can be asked for again.
const stopWait = 10_000
// The gap between the chat's last row, scrolled to the end, and the composer over it: wider from this window width.
const insetGap = { narrow: 12, wide: 16, from: 640 }

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
  const draft = useRef<ThreadComposerDraft>(null)
  const end = useRef<ChatTimelineEnd>(null)
  const pane = useRef<HTMLDivElement>(null)
  const composer = useRef<HTMLDivElement>(null)
  const [inset, setInset] = useState(0)
  const [following, setFollowing] = useState(true)
  const [column, setColumn] = useState(() => storedColumn(readStored(COLUMN_KEY, null)))
  // The column's full width, in rem: the pane less its side padding.
  const [room, setRoom] = useState(0)

  useLayoutEffect(() => {
    const root = pane.current
    const el = composer.current
    if (!root || !el) return
    const measure = () => {
      setInset(el.offsetHeight + (window.innerWidth >= insetGap.from ? insetGap.wide : insetGap.narrow))
      const style = getComputedStyle(el)
      setRoom((root.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight)) / rootRemPx())
    }
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(root)
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  const keepColumn = useCallback((rem: number) => {
    setColumn(rem)
    writeStored(COLUMN_KEY, rem)
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

  // The message whose Edit from here confirm is open, by its uuid, and why Edit is off for the app's run.
  const [editing, setEditing] = useState<string | null>(null)
  const [rewindOff, setRewindOff] = useState('')
  const readRewindOff = useCallback(() => {
    api.rewindOff().then(setRewindOff, () => undefined)
  }, [])
  useEffect(readRewindOff, [readRewindOff])
  const closeEdit = useCallback(() => {
    setEditing(null)
    readRewindOff()
  }, [readRewindOff])
  const folder = session.worktree || project?.dir || ''
  const edit: ChatEdit | undefined = session.hosted
    ? {
        blocked: (uuid) => editBlocked(uuid, rewindOff, timeline),
        open: editing,
        onOpen: setEditing,
        confirm: (uuid) => (
          <ChatEditConfirm
            thread={session.id}
            message={uuid}
            worktree={session.worktree}
            draftEmpty={() => draft.current?.empty() ?? true}
            onRewound={(text, replace) => {
              setEditing(null)
              if (replace) draft.current?.replace(text)
            }}
            onClose={closeEdit}
          />
        ),
      }
    : undefined

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
    <div ref={pane} data-chat className="relative flex h-full min-h-0 flex-col bg-app" style={{ '--chat-column': `${column}rem` } as CSSProperties} onKeyDown={onKeyDown}>
      {empty && loading && <Loading />}
      {empty && !loading && error && !session.hosted && <p className="flex flex-1 items-center justify-center px-4 text-center text-small text-dim">{error}</p>}
      {empty && !loading && error && session.hosted && <Notice title="The chat did not load" hint={error} centered />}
      {empty && !loading && !error && <div className="flex-1" />}
      {!empty && (
        <ChatTimeline
          id={session.id}
          items={timeline.items}
          folder={folder}
          inset={inset}
          endRef={end}
          onFollowing={setFollowing}
          onReply={replyTo}
          onQuote={quote}
          onFork={fork}
          edit={edit}
        />
      )}
      {/* The chat runs on under the composer; only the composer itself takes the pointer. */}
      <div ref={composer} className="chat-over pointer-events-none absolute inset-x-0 bottom-0 z-20 pt-1.5 pb-4 sm:pt-2 sm:pb-5">
        <div className="chat-column">
          <div className="pointer-events-auto relative">
            {!empty && <ChatEndPill shown={!following} queued={timeline.queued.length} onClick={() => end.current?.scrollToEnd()} />}
            {!empty && error && (
              <div className="glass mx-auto mb-2 w-fit max-w-full rounded-lg px-2 py-0.5">
                <ChatNotice text={error} error />
              </div>
            )}
            <ThreadComposer inputRef={input} draftRef={draft} reply={reply} onReply={setReply} session={session} timeline={timeline} events={events} stopping={stopping} onStop={stop} onSendNow={sendNow} onEscape={escape} />
          </div>
        </div>
      </div>
      {room > COLUMN_DEFAULT_REM && (
        <div className="chat-over pointer-events-none absolute inset-0 z-21">
          <div className="chat-column relative h-full">
            <ChatColumnHandle side="left" pane={pane} room={room} column={column} onColumn={keepColumn} />
            <ChatColumnHandle side="right" pane={pane} room={room} column={column} onColumn={keepColumn} />
          </div>
        </div>
      )}
    </div>
  )
}
