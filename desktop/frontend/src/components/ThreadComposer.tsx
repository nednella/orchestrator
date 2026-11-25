import { useCallback, useEffect, useRef, useState } from 'react'
import type { RefObject } from 'react'
import { useAgentos } from '../AgentosContext'
import type { ThreadDraft } from '../AgentosContext'
import { api } from '../api'
import { composerAction, replyLabel, replyPrefix, sendText, unseenDrafts } from '../chatComposer'
import type { ComposerAction, EscapeAction } from '../chatComposer'
import type { Timeline } from '../chatTimeline'
import { EMPTY_DRAFT, appendText } from '../composerMarkdown'
import { EFFORTS, modelName } from '../model'
import type { ChatEvent, Pending, Session } from '../types'
import { useFocusRequest } from '../useFocusRequest'
import { ChatQueued } from './ChatQueued'
import { Composer } from './Composer'
import type { ComposerInput } from './ComposerEditor'
import { Icon } from './Icon'

// A send that gets no answer in this long lets the composer send again.
const sendWait = 15_000

// onEscape does what Esc does in the thread's chat, given whether a picker of the composer is open, and says what that was.
type ThreadComposerProps = {
  inputRef: RefObject<ComposerInput | null>
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

// The host turns down what waits with the message, and Claude does what it says instead.
const WAITING_PLACEHOLDER: Record<Pending['kind'], string> = {
  tool: 'A message now declines the tool and tells Claude why',
  question: 'A message now skips the question and tells Claude what to do instead',
  plan: 'A message now turns down the plan and tells Claude what to do instead',
}

const SEND_LABEL: Record<ComposerAction, string> = { send: 'Send', queue: 'Queue', deny: 'Deny and send', stop: 'Stop' }

export function ThreadComposer({ inputRef: input, reply, onReply, session, timeline, events, stopping, onStop, onSendNow, onEscape }: ThreadComposerProps) {
  const { threadDraft, setThreadDraft, setDraftsSeen, resumeSession, report } = useAgentos('threadDraft', 'setThreadDraft', 'setDraftsSeen', 'resumeSession', 'report')
  const { id } = session
  const [current, setCurrent] = useState(() => threadDraft(id))
  const latest = useRef(current)
  const [sending, setSending] = useState(false)

  const update = useCallback(
    (next: ThreadDraft) => {
      if (next.seen !== latest.current.seen) setDraftsSeen(id, next.seen)
      latest.current = next
      setCurrent(next)
      setThreadDraft(id, next.draft)
    },
    [id, setThreadDraft, setDraftsSeen],
  )

  useEffect(() => {
    const unseen = unseenDrafts(events, latest.current.seen)
    if (unseen.length === 0) return
    update({ draft: unseen.reduce((d, e) => appendText(d, e.text ?? ''), latest.current.draft), seen: unseen[unseen.length - 1].seq })
  }, [events, update])

  const focusInput = useCallback(() => input.current?.focus(), [])
  useFocusRequest('composer', focusInput)

  if (session.state === 'ended') {
    return (
      <div className="flex items-center justify-center gap-3 py-2 text-small text-dim">
        This thread has ended
        {session.resumable && (
          <button className="btn" onClick={() => report(() => resumeSession(id))}>
            Resume
          </button>
        )}
      </div>
    )
  }

  const empty = current.draft.text.trim() === ''
  const action = composerAction(timeline, empty)
  const model = modelName(session.model)
  const effort = EFFORTS[session.effort] ?? session.effort

  const send = () => {
    const typed = sendText(latest.current.draft.text)
    if (sending || typed.trim() === '') return
    const text = (reply === null ? '' : replyPrefix(reply)) + typed
    setSending(true)
    const release = setTimeout(() => setSending(false), sendWait)
    const sent = latest.current.draft
    update({ ...latest.current, draft: EMPTY_DRAFT })
    onReply(null)
    report(async () => {
      try {
        await api.chatSend(id, text)
      } catch (err) {
        const now = latest.current.draft
        update({ ...latest.current, draft: now.text.trim() === '' ? sent : appendText(now, typed) })
        if (reply !== null) onReply(reply)
        throw err
      } finally {
        clearTimeout(release)
        setSending(false)
      }
    })
  }

  const placeholder = timeline.pending ? WAITING_PLACEHOLDER[timeline.pending.kind] : timeline.turnOpen ? 'Claude takes in a message at its next step' : 'Message Claude'

  return (
    <Composer
      inputRef={input}
      value={current.draft}
      label="Message"
      placeholder={placeholder}
      canSend={!empty}
      sendLabel={SEND_LABEL[action]}
      sendText={action === 'deny' ? SEND_LABEL.deny : undefined}
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
      right={
        (model || effort) && (
          <span className="inline-flex h-7 flex-none items-center gap-1.5 px-2 text-small font-medium text-soft" title="The thread's model and reasoning effort">
            {model && <span className="max-w-24 truncate">{model}</span>}
            {effort && <span className="max-w-24 truncate text-dim">{effort}</span>}
          </span>
        )
      }
      sending={sending}
      stop={action === 'stop' && !sending ? { stopping, onStop } : undefined}
      onChange={(next) => update({ ...latest.current, draft: next })}
      onKeyDown={(e) => {
        if (e.key === 'Escape' && onEscape(false) === 'blur') input.current?.blur()
      }}
      onSend={send}
    />
  )
}
