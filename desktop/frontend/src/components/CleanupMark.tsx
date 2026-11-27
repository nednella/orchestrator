import { Brush, TriangleAlert } from 'lucide-react'
import { cleanupNote } from '../threads'
import type { Thread } from '../types'

type CleanupMarkProps = { thread: Thread }

export function CleanupMark({ thread }: CleanupMarkProps) {
  if (thread.cleanup !== 'blocked' && thread.cleanup !== 'ask') return null
  const blocked = thread.cleanup === 'blocked'
  const note = cleanupNote(thread)
  return (
    <span role="img" aria-label={note} title={note} className={`relative flex flex-none ${blocked ? 'text-danger' : 'text-soft'}`}>
      {blocked ? <TriangleAlert size={12} aria-hidden="true" /> : <Brush size={12} aria-hidden="true" />}
    </span>
  )
}
