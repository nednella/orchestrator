import { useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import type { Note } from '../types'
import { useListNav, isTyping } from '../useListNav'
import { useScrollCursorIntoView } from '../useScrollCursorIntoView'
import { Collapse } from './Collapse'
import { Icon } from './Icon'
import { ImageViewer } from './ImageViewer'
import { NoteCard } from './NoteCard'
import { NoteJot } from './NoteJot'
import { Notice } from './Notice'

const newestFirst = (x: Note, y: Note) => y.createdAt - x.createdAt

export function NotesView() {
  const a = useAgentos('notes', 'report', 'setNotePinned', 'setNoteArchived')
  const [archivedOpen, setArchivedOpen] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [viewing, setViewing] = useState<string | null>(null)
  const root = useRef<HTMLDivElement>(null)
  const list = useRef<HTMLDivElement>(null)

  const { active, archived } = useMemo(
    () => ({
      active: a.notes.filter((n) => !n.archived).sort((x, y) => Number(y.pinned) - Number(x.pinned) || newestFirst(x, y)),
      archived: a.notes.filter((n) => n.archived).sort(newestFirst),
    }),
    [a.notes],
  )

  const visible = archivedOpen ? [...active, ...archived] : active

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
    <div ref={root} tabIndex={-1} className="flex h-full min-h-0 flex-col outline-none" onKeyDown={onKeyDown}>
      <div className="flex-none px-2 pt-2 pb-1">
        <NoteJot />
      </div>
      <div ref={list} className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-2 pt-1 pb-3">
        {a.notes.length === 0 && <Notice title="Nothing jotted yet" hint="Half ideas, things to ask, bugs you noticed. Start a thread or file an issue from any note later." centered />}
        {active.map(card)}
        {archived.length > 0 && (
          <>
            <button className="flex items-center gap-1.5 px-1 py-1 text-small text-dim hover:text-ink" aria-expanded={archivedOpen} onClick={() => setArchivedOpen(!archivedOpen)}>
              <span style={{ transform: archivedOpen ? 'none' : 'rotate(-90deg)' }}>
                <Icon name="chevron" size={12} />
              </span>
              Archived ({archived.length})
            </button>
            <Collapse open={archivedOpen}>
              <div className="flex flex-col gap-2">{archived.map(card)}</div>
            </Collapse>
          </>
        )}
      </div>
      {viewing && <ImageViewer url={viewing} onClose={() => setViewing(null)} />}
    </div>
  )
}
