import { useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import type { Issue } from '../types'
import { useFocusRequest } from '../useFocusRequest'
import { AutoTextarea } from './AutoTextarea'
import { Icon } from './Icon'
import { Keycap } from './Keycap'
import { SegmentedControl } from './SegmentedControl'
import { TypeMark } from './TypeMark'

const PICK_LIMIT = 8
const TOKEN = /(^|\s)#(\w*)$/

const matches = (issue: Issue, query: string) => (/^\d+$/.test(query) ? String(issue.number).startsWith(query) : issue.title.toLowerCase().includes(query))

export function HomeComposer() {
  const { issues, composerKind, setComposerKind, newSession, newChat, startIssue, typeInto, report } = useAgentos('issues', 'composerKind', 'setComposerKind', 'newSession', 'newChat', 'startIssue', 'typeInto', 'report')
  const input = useRef<HTMLTextAreaElement>(null)
  const [text, setText] = useState('')
  const [caret, setCaret] = useState(0)
  const [picked, setPicked] = useState<number | null>(null)
  const [cursor, setCursor] = useState(0)
  const [dismissed, setDismissed] = useState(false)
  const [sending, setSending] = useState(false)

  useFocusRequest('composer', input)

  const token = TOKEN.exec(text.slice(0, caret))
  const query = token?.[2].toLowerCase()
  const candidates = useMemo(
    () => (query === undefined || dismissed ? [] : issues.filter((i) => matches(i, query)).slice(0, PICK_LIMIT)),
    [issues, query, dismissed],
  )
  const pickedIssue = issues.find((i) => i.number === picked)
  const chat = composerKind === 'chat'

  const edit = (el: HTMLTextAreaElement) => {
    setText(el.value)
    setCaret(el.selectionStart)
    setCursor(0)
    setDismissed(false)
  }

  const pick = (number: number) => {
    if (!token) return
    const start = caret - token[2].length - 1
    setText((text.slice(0, start) + text.slice(caret)).replace(/ {2,}/g, ' '))
    setCaret(start)
    setPicked(number)
    requestAnimationFrame(() => input.current?.setSelectionRange(start, start))
  }

  const send = () => {
    if (sending) return
    report(async () => {
      setSending(true)
      try {
        const extra = text.trim()
        if (picked !== null) {
          const session = await startIssue(picked, '')
          if (extra) await typeInto(session.id, extra)
        } else if (chat) {
          const session = await newChat('')
          if (extra) await typeInto(session.id, extra)
        } else {
          await newSession('', extra)
        }
        setText('')
        setPicked(null)
      } finally {
        setSending(false)
      }
    })
  }

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (candidates.length > 0) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault()
        setCursor((c) => (e.key === 'ArrowDown' ? Math.min(c + 1, candidates.length - 1) : Math.max(c - 1, 0)))
        return
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault()
        pick(candidates[cursor].number)
        return
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        e.stopPropagation()
        setDismissed(true)
        return
      }
    }
    if (e.key === 'Escape') {
      e.currentTarget.blur()
      return
    }
    if (e.key !== 'Enter' || e.shiftKey || e.nativeEvent.isComposing) return
    e.preventDefault()
    send()
  }

  return (
    <div className="flex-none border-t border-line px-4 py-3">
      <div className="@container relative mx-auto w-full max-w-3xl">
        {candidates.length > 0 && (
          <div
            className="fade-in absolute right-0 bottom-full left-0 mb-1.5 max-h-60 overflow-y-auto rounded-md border border-line-strong bg-raised py-1"
            style={{ zIndex: 'var(--z-popup)' }}
            role="listbox"
            aria-label="Issues"
          >
            {candidates.map((issue, i) => (
              <button
                key={issue.number}
                role="option"
                aria-selected={i === cursor}
                className="row h-8 items-center gap-2 px-3"
                data-selected={i === cursor}
                onMouseDown={(e) => {
                  e.preventDefault()
                  pick(issue.number)
                }}
              >
                <span className="mono w-10 flex-none text-small text-dim">{issue.number}</span>
                <TypeMark type={issue.type} />
                <span className="min-w-0 flex-1 truncate text-body">{issue.title}</span>
                {issue.sessionId && <span className="flex-none text-small text-dim">has a thread</span>}
              </button>
            ))}
          </div>
        )}
        <div className="composer">
          {pickedIssue && (
            <div className="flex px-2.5 pt-2">
              <span className="flex min-w-0 items-center gap-2 rounded-md border border-line-strong px-2 py-0.5 text-small">
                <span className="mono flex-none text-dim">{pickedIssue.number}</span>
                <TypeMark type={pickedIssue.type} />
                <span className="min-w-0 truncate">{pickedIssue.title}</span>
                <button className="flex-none text-dim hover:text-ink" aria-label="Remove issue" onClick={() => setPicked(null)}>
                  <Icon name="close" size={11} />
                </button>
              </span>
            </div>
          )}
          <AutoTextarea
            taRef={input}
            value={text}
            aria-label="New thread"
            placeholder={pickedIssue ? 'Add to the issue’s command (optional)' : chat ? 'Ask about the project…' : 'Start a task…'}
            className="px-3 pt-2.5 pb-1 text-body"
            onChange={(e) => edit(e.currentTarget)}
            onSelect={(e) => setCaret(e.currentTarget.selectionStart)}
            onKeyDown={onKeyDown}
          />
          <div className="composer-bar">
            {!pickedIssue && (
              <SegmentedControl
                label="Kind of thread"
                options={[
                  { value: 'session', label: 'Task' },
                  { value: 'chat', label: 'Chat' },
                ]}
                value={composerKind}
                onChange={setComposerKind}
              />
            )}
            <span className="ml-1 hidden min-w-0 items-center gap-3 truncate text-label text-dim @lg:flex">
              <span>
                <Keycap>#</Keycap> picks an issue
              </span>
              <span>
                <Keycap>⇧Enter</Keycap> new line
              </span>
            </span>
            <button className="btn btn-accent ml-auto h-7 px-3" disabled={sending} onMouseDown={(e) => e.preventDefault()} onClick={send}>
              Start
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
