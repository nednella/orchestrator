import { Archive, ArchiveRestore, CircleDot, MessageSquarePlus, Pin, Trash2 } from 'lucide-react'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useOrchestrator, useOrchestratorSelector } from '../OrchestratorContext'
import { confirmed } from '../confirm'
import { imageFiles, readImage } from '../images'
import type { Note } from '../types'
import { Ago } from './Ago'
import { AutoTextarea } from './AutoTextarea'
import { NoteImages } from './NoteImages'
import { NoteText } from './NoteText'

type NoteCardProps = {
  note: Note
  cursor: boolean
  editing: boolean
  onEdit(): void
  onStopEditing(refocus: boolean): void
  onOpenImage(url: string): void
}

const FOLD_LINES = 10
const AUTOSAVE_MS = 700

export function NoteCard({ note, cursor, editing, onEdit, onStopEditing, onOpenImage }: NoteCardProps) {
  const a = useOrchestrator('report', 'updateNote', 'setNotePinned', 'setNoteArchived', 'addNoteImage', 'removeNoteImage', 'deleteNote', 'noteToIssue', 'noteToThread')
  const [expanded, setExpanded] = useState(false)
  const [overflowing, setOverflowing] = useState(false)
  const body = useRef<HTMLDivElement>(null)
  const timer = useRef(0)
  const cancelled = useRef(false)
  const finishing = useRef(false)
  const title = note.text.split('\n')[0]
  const hasRepo = useOrchestratorSelector((state) => Boolean(state.project?.repo))
  const canFile = hasRepo && note.text.trim() !== ''

  useLayoutEffect(() => {
    const el = body.current
    if (el && !expanded) setOverflowing(el.scrollHeight > el.clientHeight + 1)
  }, [note.text, expanded, editing])

  const fileIssue = () =>
    a.report(() =>
      confirmed(
        { title: 'File this note as an issue?', body: [`It becomes the issue “${title}”${note.images.length > 0 ? ', with its pictures' : ''}. The note is deleted afterwards.`], confirm: 'File issue', tone: 'primary' },
        () => a.noteToIssue(note.id),
      ),
    )
  const remove = () => a.report(() => confirmed({ title: 'Delete this note?', confirm: 'Delete', tone: 'danger' }, () => a.deleteNote(note.id)))

  const save = (text: string) => {
    if ((text.trim() || note.images.length > 0) && text.trim() !== note.text) a.report(() => a.updateNote(note.id, text))
  }

  useEffect(() => {
    if (editing) finishing.current = false
  }, [editing])

  const finish = (el: HTMLTextAreaElement, refocus: boolean) => {
    if (finishing.current) return
    finishing.current = true
    clearTimeout(timer.current)
    if (!cancelled.current) save(el.value)
    cancelled.current = false
    onStopEditing(refocus)
  }

  const attach = (files: File[]) => {
    if (files.length === 0) return false
    a.report(async () => {
      for (const image of await Promise.all(files.map(readImage))) await a.addNoteImage(note.id, image)
    })
    return true
  }

  if (editing) {
    return (
      <article className="note-card flex flex-col gap-2 p-3" data-cursor={cursor} onDragOver={(e) => e.preventDefault()} onDrop={(e) => attach(imageFiles(e.dataTransfer)) && e.preventDefault()}>
        <AutoTextarea
          autoFocus
          defaultValue={note.text}
          aria-label="Edit note"
          className="rounded-lg border border-(--border-rule) bg-(--bg-app) px-2.5 py-1.5 text-body outline-none focus:border-(--accent)"
          onFocus={(e) => e.currentTarget.setSelectionRange(note.text.length, note.text.length)}
          onPaste={(e) => attach(imageFiles(e.clipboardData)) && e.preventDefault()}
          onInput={(e) => {
            const el = e.currentTarget
            clearTimeout(timer.current)
            timer.current = window.setTimeout(() => save(el.value), AUTOSAVE_MS)
          }}
          onBlur={(e) => finish(e.currentTarget, false)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              e.stopPropagation()
              cancelled.current = true
              finish(e.currentTarget, true)
            }
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              finish(e.currentTarget, true)
            }
          }}
        />
        <NoteImages urls={note.images} onOpen={onOpenImage} onRemove={(url) => a.report(() => a.removeNoteImage(note.id, url))} />
        <p className="text-label text-soft">
          Saves as you type. Enter finishes, Esc discards the pending change, paste an image to attach it.
        </p>
      </article>
    )
  }

  const iconButton = 'btn btn-ghost h-7 w-7 justify-center px-0'
  const textButton = 'btn btn-ghost h-7 px-2'
  return (
    <article className="note-card group p-3" data-cursor={cursor} style={{ opacity: note.archived ? 0.6 : 1 }}>
      <div
        role="button"
        tabIndex={0}
        aria-label="Edit note"
        title="Click to edit"
        className="cursor-text text-left"
        onClick={onEdit}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && e.target === e.currentTarget) {
            e.preventDefault()
            onEdit()
          }
        }}
      >
        <div
          ref={body}
          className="text-body leading-relaxed break-words whitespace-pre-wrap"
          style={expanded ? undefined : { display: '-webkit-box', WebkitLineClamp: FOLD_LINES, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}
          hidden={note.text === ''}
        >
          <div>
            <NoteText text={note.text} />
          </div>
        </div>
      </div>
      {(overflowing || expanded) && (
        <button className="mt-1 text-small text-soft hover:text-ink" onClick={() => setExpanded(!expanded)}>
          {expanded ? 'Show less' : 'Show more'}
        </button>
      )}
      {note.images.length > 0 && (
        <div className="mt-2">
          <NoteImages urls={note.images} onOpen={onOpenImage} />
        </div>
      )}
      <div className="mt-2 flex min-h-7 flex-wrap items-center gap-x-1 gap-y-1">
        <span className="mr-auto text-small text-soft">
          <Ago at={note.createdAt} />
        </span>
        <span className="note-actions">
          <button className={textButton} title="Start a thread from this note" onClick={() => a.report(() => a.noteToThread(note.id))}>
            <MessageSquarePlus size={14} aria-hidden="true" />
            Start thread
          </button>
          {canFile && (
            <button className={textButton} title="File the note as an issue" onClick={fileIssue}>
              <CircleDot size={14} aria-hidden="true" />
              File as issue
            </button>
          )}
          <button
            className={iconButton}
            title={note.archived ? 'Restore from the archive (E)' : 'Archive (E)'}
            aria-label={note.archived ? 'Restore note' : 'Archive note'}
            onClick={() => a.report(() => a.setNoteArchived(note.id, !note.archived))}
          >
            {note.archived ? <ArchiveRestore size={14} aria-hidden="true" /> : <Archive size={14} aria-hidden="true" />}
          </button>
          <button className={iconButton} title="Delete" aria-label="Delete note" onClick={remove}>
            <Trash2 size={14} aria-hidden="true" />
          </button>
        </span>
        {!note.archived && (
          <button
            className={`${iconButton} -mr-1.5 ${note.pinned ? 'text-(--accent)' : ''}`}
            title={note.pinned ? 'Pinned. Click to unpin (P)' : 'Pin to the top (P)'}
            aria-label={note.pinned ? 'Unpin note' : 'Pin note'}
            aria-pressed={note.pinned}
            onClick={() => a.report(() => a.setNotePinned(note.id, !note.pinned))}
          >
            <Pin size={14} fill={note.pinned ? 'currentColor' : 'none'} aria-hidden="true" />
          </button>
        )}
      </div>
    </article>
  )
}
