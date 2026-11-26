import type { Timeline, TimelineItem } from './chatTimeline'
import type { ConfirmRequest } from './confirmQueue'
import type { FilesRewind } from './types'

// editBlocked says why Edit from here is off for the owner's message with uuid, its uuid in Claude's conversation, ''
// when the owner may edit from it. off is why Edit from here is off for the app's run, '' while it works.
export function editBlocked(uuid: string, off: string, { turnOpen, pending, queued }: Pick<Timeline, 'turnOpen' | 'pending' | 'queued'>): string {
  if (off) return off
  if (uuid === '') return "Claude's conversation does not hold this message as sent"
  if (turnOpen || pending) return 'Edit from here when the turn ends'
  if (queued.length > 0) return 'Edit from here once the queued messages are sent'
  return ''
}

// changesLabel is what a rewind of the files would change, as "3 files: +12 −4".
export function changesLabel({ files, insertions, deletions }: Pick<FilesRewind, 'files' | 'insertions' | 'deletions'>): string {
  return `${files.length} ${files.length === 1 ? 'file' : 'files'}: +${insertions} −${deletions}`
}

// laterMessages is how many messages, the owner's, Claude's replies and child results, come after the owner's message
// with uuid: what a rewind to before it takes from the chat besides the message itself.
export function laterMessages(items: TimelineItem[], uuid: string): number {
  const at = items.findIndex((i) => i.kind === 'user' && i.uuid === uuid)
  if (at === -1) return 0
  return items.slice(at + 1).filter((i) => i.kind === 'user' || i.kind === 'reply' || i.kind === 'childResult').length
}

// editRequest is the confirm of Edit from here, with later messages after it. offer is what the thread's files may do,
// null for a thread in the project folder, whose files are never rewound: the box shows only when they are offered, and
// is off with the offer's reason.
export function editRequest(later: number, offer: FilesRewind | null): Omit<ConfirmRequest, 'id'> {
  const goes = later === 0 ? [] : [`${later} later ${later === 1 ? 'message goes' : 'messages go'}.`]
  const files = offer?.offered ? { label: "Also undo Claude's file changes since then", hint: 'Shows the files first', checked: false, ...(offer.reason ? { disabled: offer.reason } : {}) } : undefined
  return {
    title: 'Edit from here?',
    body: ['Rewinds the chat to before this message. Its text goes back in the composer, and you send it.', ...goes],
    ...(files ? { checkbox: files } : {}),
    confirm: 'Rewind',
    tone: 'primary',
  }
}

// undoRequest is the second confirm of an Edit that undoes the files: the dry run's files, as shown, and its lines.
export function undoRequest(preview: Pick<FilesRewind, 'files' | 'insertions' | 'deletions'>, shown: string[]): Omit<ConfirmRequest, 'id'> {
  return {
    title: 'Undo these file changes?',
    body: ['Files changed by commands are not undone. Edits made outside Claude since then are overwritten.', changesLabel(preview)],
    list: shown,
    confirm: 'Rewind and undo',
    tone: 'danger',
  }
}
