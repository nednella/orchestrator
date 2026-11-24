import { cleanupNote } from '../threads'
import type { Session } from '../types'
import { Icon } from './Icon'

type CleanupMarkProps = { session: Session }

export function CleanupMark({ session }: CleanupMarkProps) {
  if (session.cleanup !== 'blocked' && session.cleanup !== 'ask') return null
  const blocked = session.cleanup === 'blocked'
  return (
    <span role="img" aria-label={cleanupNote(session)} className={`flex flex-none ${blocked ? 'text-danger' : 'text-soft'}`}>
      <Icon name={blocked ? 'alert' : 'broom'} size={12} />
    </span>
  )
}
