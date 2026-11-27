import { ChevronRight, Search } from 'lucide-react'
import { useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import type { Note } from '../types'
import { useListNav, isTyping } from '../useListNav'
import { useScrollCursorIntoView } from '../useScrollCursorIntoView'
import { Collapse } from './Collapse'
import { ImageViewer } from './ImageViewer'
import { NoteCard } from './NoteCard'
import { NoteJot } from './NoteJot'
import { Notice } from './Notice'

const newestFirst = (x: Note, y: Note) => y.createdAt - x.createdAt

export function NotesView() {
  const a = useOrchestrator('notes', 'report', 'setNotePinned', 'setNoteArchived')
  const [query, setQuery] = useState('')
  const [archivedOpen, setArchivedOpen] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [viewing, setViewing] = useState<string | null>(null)
  const root = useRef<HTMLDivElement>(null)
  const list = useRef<HTMLDivElement>(null)

  const { active, archived } = useMemo(() => {
    const matches = a.notes.filter((n) => n.text.toLowerCase().includes(query.trim().toLowerCase()))
    return {
      active: matches.filter((n) => !n.archived).sort((x, y) => Number(y.pinned) - Number(x.pinned) || newestFirst(x, y)),
      archived: matches.filter((n) => n.archived).sort(newestFirst),
    }
  }, [a.notes, query])

  const showArchived = archivedOpen || Boolean(query)
  const visible = showArchived ? [...active, ...archived] : active

  const listNav = useListNav(
    visible.map((n) => n.id),
    {
      onEnter(index) {
        if (visible[index]) setEditingId(visible[index].id)
      },
    },
  )

  useScrollCursorIntoView(list, listNav.cursorKey)

  const onKeyDown = (e: KeyboardEvent) => {
    if (listNav.handle(e)) {
      e.preventDefault()
      return
    }
    const note = visible[listNav.cursor]
    if (!note || isTyping(e.target) || e.metaKey || e.ctrlKey || e.altKey) return
    if (e.key === 'p' && !note.archived) a.report(() => a.setNotePinned(note.id, !note.pinned))
    else if (e.key === 'e') a.report(() => a.setNoteArchived(note.id, !note.archived))
    else return
    e.preventDefault()
  }

  const card = (note: Note) => (
    <NoteCard
      key={note.id}
      note={note}
      cursor={listNav.cursorKey === note.id}
      editing={editingId === note.id}
      onEdit={() => {
        listNav.setCursorKey(note.id)
        setEditingId(note.id)
      }}
      onStopEditing={(refocus) => {
        setEditingId(null)
        if (refocus) root.current?.focus()
      }}
      onOpenImage={setViewing}
    />
  )

  return (
    <div ref={root} tabIndex={-1} data-section="notes" className="mx-auto flex h-full min-h-0 w-full max-w-3xl flex-col outline-none" onKeyDown={onKeyDown}>
      <div className="flex flex-none flex-col gap-1.5 px-2 pt-2 pb-1">
        <NoteJot />
        {a.notes.length > 0 && (
          <div className="relative">
            <span className="pointer-events-none absolute top-2 left-2.5 text-soft">
              <Search size={14} aria-hidden="true" />
            </span>
            <input
              value={query}
              placeholder="Search notes"
              aria-label="Search notes"
              spellCheck={false}
              className="h-8 w-full rounded-lg border border-(--border-rule) bg-(--bg-card) pr-2.5 pl-8 text-small outline-none focus:border-(--accent)"
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key !== 'Escape') return
                e.stopPropagation()
                if (query) setQuery('')
                else e.currentTarget.blur()
              }}
            />
          </div>
        )}
      </div>
      <div ref={list} className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-2 pt-1 pb-3">
        {a.notes.length === 0 && <Notice title="No notes yet" hint="Write down an idea, a question or a bug. You can start a thread or file an issue from any note later." centered />}
        {a.notes.length > 0 && visible.length === 0 && <Notice title="No notes match" hint="Clear the search to see them all." centered />}
        {active.map(card)}
        {archived.length > 0 && (
          <>
            <button className="flex items-center gap-1.5 px-1 py-1 text-small text-soft hover:text-ink" aria-expanded={showArchived} onClick={() => setArchivedOpen(!archivedOpen)}>
              <span className={`flex transition-transform motion-reduce:transition-none ${showArchived ? 'rotate-90' : ''}`}>
                <ChevronRight size={14} aria-hidden="true" />
              </span>
              Archived ({archived.length})
            </button>
            <Collapse open={showArchived}>
              <div className="flex flex-col gap-2">{archived.map(card)}</div>
            </Collapse>
          </>
        )}
      </div>
      {viewing && <ImageViewer url={viewing} onClose={() => setViewing(null)} />}
    </div>
  )
}
