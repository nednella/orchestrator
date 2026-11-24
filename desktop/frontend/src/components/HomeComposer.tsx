import { useEffect, useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import type { ComposerMode } from '../AgentosContext'
import { imageFiles, readImage } from '../images'
import type { PreviewImage } from '../images'
import type { Issue } from '../types'
import { useFocusRequest } from '../useFocusRequest'
import { Composer } from './Composer'
import { ComposerAccessChip } from './ComposerAccessChip'
import { ComposerModelChip } from './ComposerModelChip'
import { Icon } from './Icon'
import { IssuePickList } from './IssuePickList'
import { NoteImages } from './NoteImages'
import { SegmentedControl } from './SegmentedControl'
import { TypeMark } from './TypeMark'

const PICK_LIMIT = 8
const TOKEN = /(^|\s)#(\w*)$/

const MODES: { value: ComposerMode; label: string }[] = [
  { value: 'thread', label: 'Thread' },
  { value: 'note', label: 'Note' },
]

const matches = (issue: Issue, query: string) => (/^\d+$/.test(query) ? String(issue.number).startsWith(query) : issue.title.toLowerCase().includes(query))

export function HomeComposer() {
  const a = useAgentos('issues', 'composerDraft', 'setComposerDraft', 'composerMode', 'setComposerMode', 'composerKind', 'setComposerKind', 'newSession', 'newChat', 'startIssue', 'addNote', 'report')
  const text = a.composerDraft
  const input = useRef<HTMLTextAreaElement>(null)
  const [caret, setCaret] = useState(0)
  const [picked, setPicked] = useState<number | null>(null)
  const [cursor, setCursor] = useState(-1)
  const [dismissed, setDismissed] = useState(false)
  const [model, setModel] = useState('')
  const [effort, setEffort] = useState('')
  const [images, setImages] = useState<PreviewImage[]>([])
  const sending = useRef(false)

  useFocusRequest('composer', input)
  useFocusRequest('terminal', input)

  const note = a.composerMode === 'note'
  const token = note || dismissed ? null : TOKEN.exec(text.slice(0, caret))
  const query = token?.[2].toLowerCase()
  const candidates = useMemo(() => (query === undefined ? [] : a.issues.filter((i) => matches(i, query)).slice(0, PICK_LIMIT)), [a.issues, query])
  const pickedIssue = note ? undefined : a.issues.find((i) => i.number === picked)
  const chat = a.composerKind === 'chat'
  const live = Boolean(pickedIssue?.sessionId)
  const canSend = note ? text.trim() !== '' || images.length > 0 : pickedIssue !== undefined || text.trim() !== ''

  useEffect(() => {
    if (picked !== null && !a.issues.some((i) => i.number === picked)) setPicked(null)
  }, [picked, a.issues])

  const edit = (el: HTMLTextAreaElement) => {
    a.setComposerDraft(el.value)
    setCaret(el.selectionStart)
    setCursor(-1)
    setDismissed(false)
  }

  const pick = (number: number) => {
    if (!token) return
    const start = caret - token[2].length - 1
    a.setComposerDraft((text.slice(0, start) + text.slice(caret)).replace(/ {2,}/g, ' '))
    setCaret(start)
    setPicked(number)
    requestAnimationFrame(() => input.current?.setSelectionRange(start, start))
  }

  const openPicker = () => {
    const el = input.current
    if (!el) return
    const at = el.selectionStart
    const lead = at > 0 && !/\s/.test(text[at - 1]) ? ' #' : '#'
    a.setComposerDraft(text.slice(0, at) + lead + text.slice(at))
    setCaret(at + lead.length)
    setDismissed(false)
    el.focus()
    requestAnimationFrame(() => el.setSelectionRange(at + lead.length, at + lead.length))
  }

  const attach = (data: DataTransfer) => {
    const files = imageFiles(data)
    if (files.length === 0) return false
    a.report(async () => {
      const read = await Promise.all(files.map(readImage))
      setImages((list) => [...list, ...read])
    })
    return true
  }

  const send = () => {
    if (sending.current || !canSend) return
    sending.current = true
    a.report(async () => {
      try {
        if (note) {
          await a.addNote(text, images)
          setImages([])
        } else {
          const start = { prompt: text.trim(), model, effort }
          if (pickedIssue) await a.startIssue(pickedIssue.number, '', false, start)
          else if (chat) await a.newChat(start)
          else await a.newSession(start)
          setPicked(null)
        }
        a.setComposerDraft('')
      } finally {
        sending.current = false
      }
    })
  }

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (token && candidates.length > 0) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault()
        setCursor((c) => (e.key === 'ArrowDown' ? Math.min(c + 1, candidates.length - 1) : Math.max(c - 1, 0)))
        return
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault()
        pick(candidates[Math.max(cursor, 0)].number)
        return
      }
    }
    if (token && e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      setDismissed(true)
      return
    }
    if (e.key === 'Escape') e.currentTarget.blur()
  }

  const placeholder = note
    ? 'Jot a note. Paste or drop images to attach them'
    : live
      ? 'The thread opens; this text is not sent'
      : pickedIssue
        ? 'Add to the issue’s command (optional)'
        : chat
          ? 'Ask anything about the project. # picks an issue'
          : 'Describe a task. # picks an issue'

  return (
    <div className="flex-none px-4 pt-2 pb-4">
      <Composer
        taRef={input}
        value={text}
        label={note ? 'New note' : 'New thread'}
        placeholder={placeholder}
        canSend={canSend}
        sendLabel={note ? 'Add note' : live ? 'Open its thread' : pickedIssue ? `Start #${pickedIssue.number}` : chat ? 'Start chat' : 'Start task'}
        sendText={note ? 'Add note' : live ? 'Open thread' : undefined}
        banner={live && <p className="px-2 py-1 text-label text-soft">#{pickedIssue?.number} already has a thread; sending opens it.</p>}
        drawer={token && <IssuePickList issues={candidates} cursor={cursor} onPick={pick} />}
        attachments={
          note
            ? images.length > 0 && <NoteImages urls={images.map((i) => i.preview)} onRemove={(url) => setImages((list) => list.filter((i) => i.preview !== url))} />
            : pickedIssue && (
                <span className="flex min-w-0 items-center gap-2 rounded-lg px-2 py-1 text-small" style={{ boxShadow: 'inset 0 0 0 1px var(--border-strong)' }}>
                  <TypeMark type={pickedIssue.type} />
                  <span className="mono flex-none text-dim">{pickedIssue.number}</span>
                  <span className="min-w-0 truncate">{pickedIssue.title}</span>
                  <button className="flex-none text-dim hover:text-ink" aria-label="Remove issue" onClick={() => setPicked(null)}>
                    <Icon name="close" size={11} />
                  </button>
                </span>
              )
        }
        left={<SegmentedControl label="Composer mode" variant="track" options={MODES} value={a.composerMode} onChange={a.setComposerMode} />}
        right={
          !note && (
            <>
              {!pickedIssue && <ComposerAccessChip kind={a.composerKind} onKind={a.setComposerKind} />}
              {!live && <ComposerModelChip model={model} effort={effort} onModel={setModel} onEffort={setEffort} />}
              <button className="composer-chip w-7 justify-center px-0 font-mono" aria-label="Pick an issue" title="Pick an issue" aria-expanded={Boolean(token)} onMouseDown={(e) => e.preventDefault()} onClick={openPicker}>
                #
              </button>
            </>
          )
        }
        onChange={(e) => edit(e.currentTarget)}
        onSelect={(e) => setCaret(e.currentTarget.selectionStart)}
        onKeyDown={onKeyDown}
        onAttach={note ? attach : undefined}
        onSend={send}
      />
    </div>
  )
}
