import { useCallback, useEffect, useRef, useState } from 'react'
import type { RefObject } from 'react'
import { useAgentos } from '../AgentosContext'
import type { ThreadDraft } from '../AgentosContext'
import { api } from '../api'
import { commandMatches, commandToken, composerAction, mentionToken, replyLabel, replyPrefix, sendText, unseenDrafts } from '../chatComposer'
import type { ComposerAction, EscapeAction } from '../chatComposer'
import type { Command, Timeline } from '../chatTimeline'
import { EMPTY_DRAFT, appendText } from '../composerMarkdown'
import { EFFORTS, modelName } from '../model'
import type { ChatEvent, Pending, Session } from '../types'
import { useFocusRequest } from '../useFocusRequest'
import { usePendingImages } from '../usePendingImages'
import { ChatQueued } from './ChatQueued'
import { Composer } from './Composer'
import type { ComposerInput } from './ComposerEditor'
import { ComposerPickList } from './ComposerPickList'
import type { PickRow } from './ComposerPickList'
import { Icon } from './Icon'
import { NoteImages } from './NoteImages'

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

// PickList is the pick list open in the composer: what it lists and what picking a row does.
type PickList = { label: string; rows: PickRow[]; empty: string; pick(index: number): void }

// Files are what the thread's folder lists for the mention, or why it did not.
type Files = { paths: string[]; error: string }

const commandRow = (c: Command): PickRow => ({ key: c.name, icon: <Icon name="terminal" size={14} />, name: `/${c.name}`, description: c.description, badge: c.argumentHint })

const fileRow = (path: string): PickRow => {
  const slash = path.lastIndexOf('/')
  return { key: path, icon: <Icon name="file" size={14} />, name: path.slice(slash + 1), description: path.slice(0, slash + 1) }
}

export function ThreadComposer({ inputRef: input, reply, onReply, session, timeline, events, stopping, onStop, onSendNow, onEscape }: ThreadComposerProps) {
  const { threadDraft, setThreadDraft, setDraftsSeen, resumeSession, report, pushToast } = useAgentos('threadDraft', 'setThreadDraft', 'setDraftsSeen', 'resumeSession', 'report', 'pushToast')
  const { id } = session
  const [current, setCurrent] = useState(() => threadDraft(id))
  const latest = useRef(current)
  const latestReply = useRef(reply)
  latestReply.current = reply
  const [sending, setSending] = useState(false)
  const pending = usePendingImages()
  const picker = useRef<HTMLInputElement>(null)
  const [caret, setCaret] = useState('')
  const [cursor, setCursor] = useState(-1)
  const [dismissed, setDismissed] = useState(false)
  const [files, setFiles] = useState<Files | null>(null)
  const command = dismissed ? null : commandToken(caret)
  const mention = dismissed || command !== null ? null : mentionToken(caret)

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

  useEffect(() => {
    if (mention === null) return
    let current = true
    api.chatFiles(id, mention).then(
      (paths) => current && setFiles({ paths, error: '' }),
      (err) => current && setFiles({ paths: [], error: String(err) }),
    )
    return () => {
      current = false
    }
  }, [id, mention])

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

  const empty = current.draft.text.trim() === '' && pending.images.length === 0
  const action = composerAction(timeline, empty)
  const model = modelName(session.model)
  const effort = EFFORTS[session.effort] ?? session.effort

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

  // replaceToken puts text where the token typed so far is, with the @ or / before it.
  const replaceToken = (token: string, text: string) => {
    input.current?.deleteBefore(token.length + 1)
    input.current?.insertLiteral(text)
    setCursor(-1)
  }

  const shownFiles = files ?? { paths: [], error: '' }
  const commands = command === null ? [] : commandMatches(timeline.commands, command)
  const pickList: PickList | null =
    command !== null
      ? {
          label: 'Commands',
          rows: commands.map(commandRow),
          empty: timeline.commands.length === 0 ? 'Commands load when the thread starts' : 'No command matches',
          pick: (i) => replaceToken(command, `/${commands[i].name} `),
        }
      : mention !== null
        ? {
            label: 'Files',
            rows: shownFiles.paths.map(fileRow),
            empty: files === null ? 'Listing the files' : shownFiles.error || 'No file matches',
            pick: (i) => replaceToken(mention, `@${shownFiles.paths[i]} `),
          }
        : null
  const highlighted = pickList ? Math.min(cursor, pickList.rows.length - 1) : -1

  const onKeyDown = (e: KeyboardEvent) => {
    if (pickList && pickList.rows.length > 0) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault()
        setCursor(e.key === 'ArrowDown' ? Math.min(highlighted + 1, pickList.rows.length - 1) : Math.max(highlighted - 1, 0))
        return
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault()
        pickList.pick(Math.max(highlighted, 0))
        return
      }
    }
    if (e.key !== 'Escape') return
    const done = onEscape(pickList !== null)
    if (done === 'close-picker') {
      e.preventDefault()
      e.stopPropagation()
      setDismissed(true)
    }
    if (done === 'blur') input.current?.blur()
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
        (model || effort) && (
          <span className="inline-flex h-7 flex-none items-center gap-1.5 px-2 text-small font-medium text-soft" title="The thread's model and reasoning effort">
            {model && <span className="max-w-24 truncate">{model}</span>}
            {effort && <span className="max-w-24 truncate text-dim">{effort}</span>}
          </span>
        )
      }
      sending={sending}
      stop={action === 'stop' && !sending ? { stopping, onStop } : undefined}
      onChange={(next) => {
        update({ ...latest.current, draft: next })
        setCursor(-1)
        setDismissed(false)
      }}
      onCaret={(before) => setCaret(before)}
      onKeyDown={onKeyDown}
      onAttach={pending.attach}
      onSend={send}
    />
  )
}
