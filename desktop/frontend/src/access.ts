import type { Access } from './types'

// AccessChoice is the access chip's two steps: Read or Write, and under Write whether Claude still asks.
export type AccessChoice = { level: 'read' | 'write'; write: 'supervised' | 'full' }

export const ACCESS_LABEL: Record<Access, string> = {
  read: 'Read',
  supervised: 'Write · Supervised',
  full: 'Write · Full access',
}

export function accessOf(choice: AccessChoice): Access {
  return choice.level === 'read' ? 'read' : choice.write
}

// choiceOf is the chip's choice for an access. Read keeps Supervised under Write, so turning to Write never gives Full
// access by itself.
export function choiceOf(access: Access): AccessChoice {
  return access === 'read' ? { level: 'read', write: 'supervised' } : { level: 'write', write: access }
}
