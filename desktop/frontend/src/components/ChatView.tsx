import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { CSSProperties, KeyboardEvent } from 'react'
import { useOrchestrator, useOrchestratorSelector, useLatestOrchestrator } from '../OrchestratorContext'
import { api, errorMessage } from '../api'
import { COLUMN_DEFAULT_REM, COLUMN_KEY, rootRemPx, storedColumn } from '../chatColumn'
import { escapeAction, lastTurnEnd, quoteMarkdown } from '../chatComposer'
import type { EscapeAction } from '../chatComposer'
import { forkDividers, forksKey, lineageDivider } from '../chatDividers'
import { editBlocked, editRequest, laterMessages, undoRequest } from '../chatEdit'
import { relative } from '../chatTools'
import { askConfirm } from '../confirm'
import { appendText, fromMarkdown } from '../composerMarkdown'
import { sideChatAsked, sideChatReason } from '../sideChat'
import { noteSideChatReason } from '../sideChatStore'
import { readStored, writeStored } from '../storage'
import type { FilesRewind, Thread } from '../types'
import { withSetup } from '../setupLine'
import { startTimeline } from '../startTimeline'
import { useChat } from '../useChat'
import { isTyping } from '../useListNav'
import { ChatChildApprovals } from './ChatChildApprovals'
import { ChatColumnHandle } from './ChatColumnHandle'
import { ChatEndPill } from './ChatEndPill'
import { ChatNotice } from './ChatNotice'
import { ChatTimeline } from './ChatTimeline'
import type { ChatEdit, ChatTimelineEnd } from './ChatTimeline'
import type { ComposerInput } from './ComposerEditor'
import { Notice } from './Notice'
import { ThreadComposer } from './ThreadComposer'
import type { ThreadComposerDraft } from './ThreadComposer'

// A stop that gets no turn end in this long can be asked for again.
const stopWait = 10_000
// The gap between the chat's last row, scrolled to the end, and the composer over it: wider from this window width.
const insetGap = { narrow: 12, wide: 16, from: 640 }

// embedded is a side chat's chat in its tab: the timeline of its own messages and the composer, with no fork from a
// message and no side chat of its own. onAsked hears whether it was sent a message of its own yet.
type ChatViewProps = { thread: Thread; embedded?: boolean; onAsked?(asked: boolean): void }

const noFiles: FilesRewind = { offered: false, reason: '', files: [], insertions: 0, deletions: 0 }

export function ChatView({ thread, embedded = false, onAsked }: ChatViewProps) {
  const { project, report, forkThread, threadDraft, setThreadDraft } = useOrchestrator('project', 'report', 'forkThread', 'threadDraft', 'setThreadDraft')
  const latest = useLatestOrchestrator()
  const starting = thread.start !== ''
  const chat = useChat(thread.id, thread.state, embedded, starting)
  // A thread that starts shows the chat it will have once its agent takes the message, and keeps it until the chat has
  // loaded, so the message does not leave and come back.
  const frozen = useRef<ReturnType<typeof startTimeline> | null>(null)
  const current = useMemo(() => (starting ? startTimeline(thread) : null), [starting, thread])
  if (current) frozen.current = current
  else if (!chat.loading) frozen.current = null
  const early = current ?? frozen.current
  const { timeline: shownTimeline, events, loading, error } = early ? { ...early, loading: false, error: '' } : chat
  const { setup, setupExit, setupShell } = thread
  const timeline = useMemo(() => (embedded ? shownTimeline : withSetup(shownTimeline, { setup, setupExit, setupShell })), [embedded, shownTimeline, setup, setupExit, setupShell])
  const empty = timeline.items.length === 0
  const forks = useOrchestratorSelector((s) => forksKey(thread.id, s.threads))
  const { id: threadId, forkOf, forkAt, parent } = thread
  const parentTitle = useOrchestratorSelector((s) => (parent ? s.threads.find((x) => x.id === parent)?.title : undefined))
  const shown = useMemo(() => {
    if (embedded) return timeline.items
    const items = forkDividers({ id: threadId, forkOf, forkAt }, JSON.parse(forks), timeline.items)
    const lineage = lineageDivider({ parent }, parentTitle === undefined ? undefined : { id: parent, title: parentTitle })
    return lineage ? [lineage, ...items] : items
  }, [embedded, threadId, forkOf, forkAt, forks, timeline.items, parent, parentTitle])

  // The right panel's Side chat row says why this thread cannot start one, which only its chat tells.
  useEffect(() => {
    if (!embedded && !loading) noteSideChatReason(threadId, sideChatReason(thread, events))
  }, [embedded, loading, threadId, thread, events])
  const asked = embedded && sideChatAsked(events)
  useEffect(() => onAsked?.(asked), [asked, onAsked])
  const lastEnd = lastTurnEnd(events)
  // The last turn end before the stop was asked for: the stop holds until a later one.
  const [stopAsked, setStopAsked] = useState<number | null>(null)
  const stopping = timeline.turnOpen && (starting || stopAsked === lastEnd)

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
      const forked = forkThread(thread.id, at, draft)
      report(() => forked)
      return forked.then(
        () => undefined,
        () => undefined,
      )
    },
    [thread.id, report, forkThread],
  )

  // Why Edit from here is off for the app's run, read again after each edit.
  const [rewindOff, setRewindOff] = useState('')
  const readRewindOff = useCallback(() => {
    api.rewindOff().then(setRewindOff, () => undefined)
  }, [])
  useEffect(readRewindOff, [readRewindOff])
  // The latest items, read when an edit is asked for, so the rows keep one onOpen while the chat streams.
  const items = useRef(timeline.items)
  items.current = timeline.items
  const editing = useRef(false)
  const { worktree } = thread
  // Edit from here asks first, and with the files box ticked shows the dry run's files and asks again. The message's
  // text then goes in the composer, after a blank line when it holds a draft.
  const editFrom = useCallback(
    (uuid: string) => {
      if (editing.current) return
      editing.current = true
      report(async () => {
        try {
          const offer = worktree ? await api.rewindFilesOffer(threadId).catch((err): FilesRewind => ({ ...noFiles, offered: true, reason: errorMessage(err) })) : null
          const asked = await askConfirm(editRequest(laterMessages(items.current, uuid), offer))
          if (!asked.ok) return
          let files = false
          if (asked.checked) {
            const preview = await api.rewindPreview(threadId, uuid)
            if (preview.reason) throw new Error(preview.reason)
            files = preview.files.length > 0
            if (files && !(await askConfirm(undoRequest(preview, preview.files.map((f) => relative(f, worktree))))).ok) return
          }
          const text = await api.rewindTo(threadId, uuid, files)
          // A view closed while the rewind ran has no composer: the text goes into the thread's saved draft.
          if (draft.current) draft.current.append(text)
          else setThreadDraft(threadId, appendText(threadDraft(threadId).draft, text))
        } finally {
          editing.current = false
          readRewindOff()
        }
      })
    },
    [threadId, worktree, report, readRewindOff, threadDraft, setThreadDraft],
  )
  const folder = thread.worktree || project?.dir || ''
  const edit: ChatEdit = { blocked: (uuid) => editBlocked(uuid, rewindOff, timeline), onOpen: editFrom }

  const stop = () => stopWith(() => api.chatInterrupt(thread.id))
  const sendNow = (message: string) => stopWith(() => api.chatSendNow(thread.id, message))

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
    <div ref={pane} data-chat data-embedded={embedded || undefined} className="relative flex h-full min-h-0 flex-col bg-app" style={{ '--chat-column': `${column}rem` } as CSSProperties} onKeyDown={onKeyDown}>
      {empty && !loading && error && <Notice title="The chat did not load" hint={error} centered />}
      {empty && (loading || !error) && <div className="flex-1" />}
      {!empty && (
        <ChatTimeline
          id={thread.id}
          items={shown}
          folder={folder}
          inset={inset}
          endRef={end}
          onFollowing={setFollowing}
          onReply={replyTo}
          onQuote={quote}
          onFork={embedded ? undefined : fork}
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
            {!embedded && <ChatChildApprovals parent={thread.id} />}
            <ThreadComposer inputRef={input} draftRef={draft} reply={reply} onReply={setReply} thread={thread} timeline={timeline} events={events} embedded={embedded} stopping={stopping} onStop={stop} onSendNow={sendNow} onEscape={escape} />
          </div>
        </div>
      </div>
      {!embedded && room > COLUMN_DEFAULT_REM && (
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
