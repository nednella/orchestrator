import { cleanupNote } from '../threads'
import type { Thread } from '../types'
import { Icon } from './Icon'

type CleanupMarkProps = { thread: Thread }

export function CleanupMark({ thread }: CleanupMarkProps) {
  if (thread.cleanup !== 'blocked' && thread.cleanup !== 'ask') return null
  const blocked = thread.cleanup === 'blocked'
  const note = cleanupNote(thread)
  return (
    <span role="img" aria-label={note} title={note} className={`relative flex flex-none ${blocked ? 'text-danger' : 'text-soft'}`}>
      <Icon name={blocked ? 'alert' : 'broom'} size={12} />
    </span>
  )
}
