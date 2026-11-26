import { useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react'
import type { Ref, RefObject } from 'react'
import { useAgentos } from '../AgentosContext'
import type { ThreadDraft } from '../AgentosContext'
import { accessDisabled } from '../access'
import { api } from '../api'
import { composerAction, replyLabel, replyPrefix, sendText, unseenDrafts } from '../chatComposer'
import type { ComposerAction, EscapeAction } from '../chatComposer'
import type { Timeline } from '../chatTimeline'
import { EMPTY_DRAFT, appendText, draftOf, fromMarkdown } from '../composerMarkdown'
import { pickCursor, pickListKey } from '../composerPick'
import { modelAlias } from '../model'
import type { ChatEvent, Session } from '../types'
import { useFocusRequest } from '../useFocusRequest'
import { usePendingImages } from '../usePendingImages'
import { usePromptHistory } from '../usePromptHistory'
import { usePromptPicks } from '../usePromptPicks'
import { ChatQueued } from './ChatQueued'
import { Composer } from './Composer'
import type { ComposerInput } from './ComposerEditor'
import { ComposerModelChip } from './ComposerModelChip'
import { ComposerPickList } from './ComposerPickList'
import { Icon } from './Icon'
import { NoteImages } from './NoteImages'

// A send that gets no answer in this long lets the composer send again.
const sendWait = 15_000

// ThreadComposerDraft is the composer's draft for the chat: whether it holds no text and no image, and ways to put text
// in its place or after it.
export type ThreadComposerDraft = { empty(): boolean; replace(text: string): void; append(text: string): void }

// onEscape does what Esc does in the thread's chat, given whether a picker of the composer is open, and says what that was.
type ThreadComposerProps = {
  inputRef: RefObject<ComposerInput | null>
  draftRef: Ref<ThreadComposerDraft>
  reply: string | null
  onReply(text: string | null): void
  session: Session
  timeline: Timeline
  events: ChatEvent[]
  stopping: boolean
  onStop(): void
  onSendNow(id: string): void
  onEscape(picker: boolean): EscapeAction
}

const SEND_LABEL: Record<ComposerAction, string> = { send: 'Send', queue: 'Queue', deny: 'Deny and send', stop: 'Stop' }

export function ThreadComposer({ inputRef: input, draftRef, reply, onReply, session, timeline, events, stopping, onStop, onSendNow, onEscape }: ThreadComposerProps) {
  const { threadDraft, setThreadDraft, setDraftsSeen, setAccess, report, pushToast } = useAgentos('threadDraft', 'setThreadDraft', 'setDraftsSeen', 'setAccess', 'report', 'pushToast')
  const { id } = session
  const [current, setCurrent] = useState(() => threadDraft(id))
  const latest = useRef(current)
  const latestReply = useRef(reply)
  latestReply.current = reply
  const [sending, setSending] = useState(false)
  const pending = usePendingImages()
  const picker = useRef<HTMLInputElement>(null)
  const [caret, setCaret] = useState({ before: '', after: '' })
  const [cursor, setCursor] = useState(-1)
  const [dismissed, setDismissed] = useState(false)
  const listFiles = useCallback((query: string) => api.chatFiles(id, query), [id])
  const pickList = usePromptPicks({
    input,
    caret,
    off: dismissed,
    commands: timeline.commands,
    noCommands: 'Commands load when the thread starts',
    listFiles,
    onPicked: () => setCursor(-1),
  })

  const update = useCallback(
    (next: ThreadDraft) => {
      if (next.seen !== latest.current.seen) setDraftsSeen(id, next.seen)
      latest.current = next
      setCurrent(next)
      setThreadDraft(id, next.draft)
    },
    [id, setThreadDraft, setDraftsSeen],
  )

  const history = usePromptHistory({
    input,
    thread: id,
    items: timeline.items,
    value: current.draft,
    pending: timeline.pending !== null,
    attachments: pending.images.length,
    menuOpen: pickList !== null,
    onRecall: (draft) => update({ ...latest.current, draft }),
  })

  useEffect(() => {
    const unseen = unseenDrafts(events, latest.current.seen)
    if (unseen.length === 0) return
    update({ draft: unseen.reduce((d, e) => appendText(d, e.text ?? ''), latest.current.draft), seen: unseen[unseen.length - 1].seq })
  }, [events, update])

  const focusInput = useCallback(() => input.current?.focus(), [])
  useFocusRequest('composer', focusInput)

  useImperativeHandle(
    draftRef,
    () => ({
      empty: () => latest.current.draft.text.trim() === '' && pending.images.length === 0,
      replace: (text: string) => {
        update({ ...latest.current, draft: draftOf(fromMarkdown(text)) })
        input.current?.focus()
      },
      append: (text: string) => {
        update({ ...latest.current, draft: appendText(latest.current.draft, text) })
        input.current?.focus()
      },
    }),
    [update, pending.images.length],
  )

  // A settled thread with a chat keeps its composer, and a message wakes it; one that ran in a terminal cannot wake.
  if (session.state === 'settled' && !session.hosted) {
    return <div className="glass mx-auto w-fit rounded-full px-3 py-1 text-small text-dim">This thread is asleep</div>
  }

  const empty = current.draft.text.trim() === '' && pending.images.length === 0
  const action = composerAction(timeline, empty)

  const send = () => {
    const typed = sendText(latest.current.draft.text)
    const images = pending.images
    if (sending || (typed.trim() === '' && images.length === 0)) return
    const text = (reply === null ? '' : replyPrefix(reply)) + typed
    setSending(true)
    const release = setTimeout(() => setSending(false), sendWait)
    const sent = latest.current.draft
    update({ ...latest.current, draft: EMPTY_DRAFT })
    pending.clear()
    onReply(null)
    report(async () => {
      try {
        const { notice } = await api.chatSend(id, text, images.map(({ base64, mime }) => ({ base64, mime })))
        if (notice) pushToast({ tone: 'info', text: notice })
      } catch (err) {
        const now = latest.current.draft
        if (typed.trim() !== '') update({ ...latest.current, draft: now.text.trim() === '' ? sent : appendText(now, typed) })
        pending.restore(images)
        if (reply !== null && latestReply.current === null) onReply(reply)
        throw err
      } finally {
        clearTimeout(release)
        setSending(false)
      }
    })
  }

  const highlighted = pickList ? pickCursor(cursor, pickList.rows.length) : -1

  const onKeyDown = (e: KeyboardEvent) => {
    const step = pickList && pickListKey(e.key, pickList.rows.length, highlighted)
    if (step) {
      e.preventDefault()
      if ('pick' in step) pickList?.pick(step.pick)
      else setCursor(step.move)
      return
    }
    history.onKeyDown(e)
    if (e.defaultPrevented || e.key !== 'Escape') return
    const done = onEscape(pickList !== null)
    if (done === 'close-picker') {
      e.preventDefault()
      e.stopPropagation()
      setDismissed(true)
    }
    if (done === 'blur') input.current?.blur()
  }

  return (
    <Composer
      inputRef={input}
      value={current.draft}
      label="Message"
      canSend={!empty}
      sendLabel={SEND_LABEL[action]}
      sendText={action === 'deny' ? SEND_LABEL.deny : undefined}
      drawer={pickList && <ComposerPickList label={pickList.label} rows={pickList.rows} cursor={highlighted} empty={pickList.empty} onPick={pickList.pick} />}
      banner={
        (reply !== null || timeline.queued.length > 0) && (
          <>
            {reply !== null && (
              <div className="flex items-center gap-2 px-3 py-1.5 text-small text-soft">
                <span className="min-w-0 flex-1 truncate">Replying to: {replyLabel(reply)}</span>
                <button className="btn btn-ghost h-5 px-1" aria-label="Cancel reply" onClick={() => onReply(null)}>
                  <Icon name="x" size={12} />
                </button>
              </div>
            )}
            {timeline.queued.length > 0 && <ChatQueued queued={timeline.queued} stopping={stopping} onSendNow={onSendNow} />}
          </>
        )
      }
      attachments={pending.images.length > 0 && <NoteImages urls={pending.images.map((i) => i.preview)} attachedTo="message" onRemove={pending.remove} />}
      left={
        <>
          <button className="composer-chip" title="Attach images" aria-label="Attach images" onMouseDown={(e) => e.preventDefault()} onClick={() => picker.current?.click()}>
            <Icon name="paperclip" size={14} />
          </button>
          <input
            ref={picker}
            type="file"
            accept="image/png,image/jpeg,image/gif,image/webp"
            multiple
            hidden
            onChange={(e) => {
              pending.attach(e.currentTarget)
              e.currentTarget.value = ''
            }}
          />
        </>
      }
      right={
        <ComposerModelChip
          model={session.model && modelAlias(session.model)}
          effort={session.effort}
          access={session.access}
          accessOff={accessDisabled(timeline.turnOpen, timeline.pending !== null, timeline.agentsRunning)}
          onAccess={(next) => report(() => setAccess(id, next))}
        />
      }
      sending={sending}
      stop={action === 'stop' && !sending ? { stopping, onStop } : undefined}
      onChange={(next) => {
        history.edited(next.text)
        update({ ...latest.current, draft: next })
        setCursor(-1)
        setDismissed(false)
      }}
      onCaret={(before, after) => setCaret((caret) => (caret.before === before && caret.after === after ? caret : { before, after }))}
      onKeyDown={onKeyDown}
      onAttach={pending.attach}
      onSend={send}
    />
  )
}
