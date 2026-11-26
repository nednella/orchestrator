import { MessageSquare, StickyNote } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'
import { commandToken } from '../chatComposer'
import type { Command } from '../chatTimeline'
import { EMPTY_DRAFT } from '../composerMarkdown'
import { doubledSpace, pickCursor, pickListKey } from '../composerPick'
import type { ComposerDraft } from '../composerMarkdown'
import type { ComposerMode } from '../OrchestratorContext'
import type { Issue } from '../types'
import { useFocusRequest } from '../useFocusRequest'
import { usePendingImages } from '../usePendingImages'
import { usePromptPicks } from '../usePromptPicks'
import { Composer } from './Composer'
import type { ComposerInput } from './ComposerEditor'
import { ComposerModelChip } from './ComposerModelChip'
import { ComposerPickList } from './ComposerPickList'
import { Icon } from './Icon'
import { IssueChips } from './IssueChips'
import { IssuePickList } from './IssuePickList'
import { NoteImages } from './NoteImages'
import { SegmentedControl } from './SegmentedControl'
import { TypeMark } from './TypeMark'

const PICK_LIMIT = 8
const CHIP_LIMIT = 3
const TOKEN = /(^|\s)#(\w*)$/

const MODES: { value: ComposerMode; label: string; icon: ReactNode }[] = [
  { value: 'thread', label: 'Thread', icon: <MessageSquare size={14} aria-hidden="true" /> },
  { value: 'note', label: 'Note', icon: <StickyNote size={14} aria-hidden="true" /> },
]

const matches = (issue: Issue, query: string) => (/^\d+$/.test(query) ? String(issue.number).startsWith(query) : issue.title.toLowerCase().includes(query))

export function HomeComposer() {
  const a = useOrchestrator('project', 'issues', 'composerDraft', 'setComposerDraft', 'composerMode', 'setComposerMode', 'composerAccess', 'setComposerAccess', 'newThread', 'startIssue', 'addNote', 'report')
  const { text } = a.composerDraft
  const input = useRef<ComposerInput>(null)
  const [caret, setCaret] = useState({ before: '', after: '' })
  const [picked, setPicked] = useState<number | null>(null)
  const [cursor, setCursor] = useState(-1)
  const [dismissed, setDismissed] = useState(false)
  const [model, setModel] = useState('')
  const [effort, setEffort] = useState('')
  const [commands, setCommands] = useState<{ project: string; list: Command[] } | null>(null)
  const pending = usePendingImages()
  const sending = useRef(false)
  const projectKey = a.project?.key ?? ''
  // null while this project's list loads, so a switch never shows the last project's commands.
  const projectCommands = commands?.project === projectKey ? commands.list : null

  const focusInput = useCallback(() => input.current?.focus(), [])
  useFocusRequest('composer', focusInput)
  useFocusRequest('thread', focusInput)

  const note = a.composerMode === 'note'
  const token = note || dismissed ? null : TOKEN.exec(caret.before)
  const query = token?.[2].toLowerCase()
  const candidates = useMemo(() => (query === undefined ? [] : a.issues.filter((i) => matches(i, query)).slice(0, PICK_LIMIT)), [a.issues, query])
  // A new thread starts in the project folder, so a mention lists its files.
  const listFiles = useCallback((q: string) => api.projectFiles(projectKey, q), [projectKey])
  const picks = usePromptPicks({
    input,
    caret,
    off: note || dismissed || token !== null,
    commands: projectCommands ?? [],
    noCommands: projectCommands ? 'Commands show once a thread has run in this project.' : 'Listing the commands',
    listFiles,
    onPicked: () => setCursor(-1),
  })
  const rows = token ? candidates.length : (picks?.rows.length ?? 0)
  const highlighted = pickCursor(cursor, rows)
  const typingCommand = !note && commandToken(caret.before) !== null
  const pickedIssue = note ? undefined : a.issues.find((i) => i.number === picked)
  const live = Boolean(pickedIssue?.sessionId)
  const canSend = note ? text.trim() !== '' || pending.images.length > 0 : pickedIssue !== undefined || text.trim() !== ''
  // The top of the queue: the issues of its first section, which the core lists first.
  const chips = useMemo(() => a.issues.filter((i) => i.section === a.issues[0]?.section).slice(0, CHIP_LIMIT), [a.issues])

  useEffect(() => {
    if (picked !== null && !a.issues.some((i) => i.number === picked)) setPicked(null)
  }, [picked, a.issues])

  // A thread that ran since the list was read may have reported a newer one.
  useEffect(() => {
    if (!typingCommand || !projectKey) return
    let current = true
    a.report(async () => {
      const list = await api.projectCommands(projectKey)
      if (current) setCommands({ project: projectKey, list })
    })
    return () => {
      current = false
    }
  }, [typingCommand, projectKey, a.report])

  const edit = (draft: ComposerDraft) => {
    a.setComposerDraft(draft)
    setCursor(-1)
    setDismissed(false)
  }

  const pick = (number: number) => {
    if (!token) return
    input.current?.deleteBefore(token[2].length + 1 + (doubledSpace(token[1], caret.after) ? 1 : 0))
    setPicked(number)
    setCursor(-1)
  }

  const pickChip = (number: number) => {
    a.setComposerMode('thread')
    setPicked(number)
    input.current?.focus()
  }

  const openPicker = () => {
    input.current?.insert(caret.before !== '' && !/\s$/.test(caret.before) ? ' #' : '#')
    input.current?.focus()
  }

  const send = () => {
    if (sending.current || !canSend) return
    sending.current = true
    const sentFrom = a.project?.name
    a.report(async () => {
      try {
        if (note) {
          await a.addNote(text, pending.images)
          pending.clear()
        } else {
          const start = { prompt: text.trim(), model, effort }
          if (pickedIssue) await a.startIssue(pickedIssue.number, '', false, start)
          else await a.newThread(start, a.composerAccess, '')
          setPicked(null)
        }
        a.setComposerDraft(EMPTY_DRAFT, sentFrom)
      } finally {
        sending.current = false
      }
    })
  }

  const onKeyDown = (e: KeyboardEvent) => {
    const open = token !== null || picks !== null
    const step = open && pickListKey(e.key, rows, highlighted)
    if (step) {
      e.preventDefault()
      if (!('pick' in step)) setCursor(step.move)
      else if (token) pick(candidates[step.pick].number)
      else picks?.pick(step.pick)
      return
    }
    if (open && e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      setDismissed(true)
      return
    }
    if (e.key === 'Escape') input.current?.blur()
  }

  return (
    <div className="flex flex-col">
      <Composer
        inputRef={input}
        value={a.composerDraft}
        label={note ? 'New note' : 'New thread'}
        placeholder={note ? 'Jot a note. Paste or drop images to attach them' : undefined}
        canSend={canSend}
        sendLabel={note ? 'Add note' : live ? 'Open its thread' : pickedIssue ? `Start #${pickedIssue.number}` : 'Start thread'}
        sendText={note ? 'Add note' : live ? 'Open thread' : undefined}
        banner={live && <p className="px-2 py-1 text-label text-soft">#{pickedIssue?.number} already has a thread; sending opens it.</p>}
        drawer={
          token ? (
            <IssuePickList issues={candidates} cursor={highlighted} onPick={pick} />
          ) : (
            picks && <ComposerPickList label={picks.label} rows={picks.rows} cursor={highlighted} empty={picks.empty} onPick={picks.pick} />
          )
        }
        attachments={
          note
            ? pending.images.length > 0 && <NoteImages urls={pending.images.map((i) => i.preview)} onRemove={pending.remove} />
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
        left={(footer) => (
          <>
            <SegmentedControl label="Composer mode" variant="track" options={MODES} shows={footer === 'compact' ? 'icon' : 'label'} value={a.composerMode} onChange={a.setComposerMode} />
            {!note && (
              <>
                <span className="mx-1.5 h-[18px] w-px flex-none bg-line" aria-hidden="true" />
                <button className="composer-chip" title="Pick an issue" aria-label="Pick an issue" data-active={Boolean(pickedIssue)} aria-expanded={Boolean(token)} onMouseDown={(e) => e.preventDefault()} onClick={openPicker}>
                  <span className="font-mono">#</span>
                  {footer === 'wide' && 'Issue'}
                </button>
              </>
            )}
          </>
        )}
        right={
          !note &&
          !live && (
            <ComposerModelChip
              model={model}
              effort={effort}
              access={pickedIssue?.access ?? a.composerAccess}
              accessOff={pickedIssue ? `The issue’s action, ${pickedIssue.actions[0]}, sets the access` : ''}
              onModel={setModel}
              onEffort={setEffort}
              onAccess={a.setComposerAccess}
            />
          )
        }
        onChange={edit}
        onCaret={(before, after) => setCaret((caret) => (caret.before === before && caret.after === after ? caret : { before, after }))}
        onKeyDown={onKeyDown}
        onAttach={note ? pending.attach : undefined}
        onSend={send}
      />
      <IssueChips issues={chips} picked={note ? null : picked} onPick={pickChip} />
    </div>
  )
}
