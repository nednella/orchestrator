import type { Timeline } from './chatTimeline'
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
