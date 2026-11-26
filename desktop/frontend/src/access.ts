import type { Access } from './types'

// AccessChoice is the access chip's two steps: Read or Write, and under Write whether Claude still asks.
export type AccessChoice = { level: 'read' | 'write'; write: 'supervised' | 'full' }

export const ACCESS_LABEL: Record<Access, string> = {
  read: 'Read',
  supervised: 'Write · Supervised',
  full: 'Write · Full access',
}

// ACCESSES are the access levels, least first, as the access controls offer them, each under its ACCESS_NAME.
export const ACCESSES: Access[] = ['read', 'supervised', 'full']

export const ACCESS_NAME: Record<Access, string> = {
  read: 'Read',
  supervised: 'Supervised',
  full: 'Full access',
}

// newThreadAccess is the access a new thread starts with: the setting, Supervised when it is unset.
export function newThreadAccess(setting: Access | ''): Access {
  return setting || 'supervised'
}

export const ACCESS_HINT: Record<Access, string> = {
  read: 'It reads the project and plans; Claude asks before it changes anything',
  supervised: 'Your Claude allow rules apply; other tools ask in the chat',
  full: 'Every tool runs without asking',
}

export function accessOf(choice: AccessChoice): Access {
  return choice.level === 'read' ? 'read' : choice.write
}

// accessDisabled is why a thread's access cannot change now, "" when it can: a turn keeps the access it started with,
// and a change restarts Claude, which would stop the subagents that run.
export function accessDisabled(turnOpen: boolean, pending: boolean, agentsRunning = false): string {
  if (turnOpen || pending) return 'Change access when the turn ends'
  return agentsRunning ? 'Change access when the subagents end' : ''
}

// choiceOf is the chip's choice for an access. Read keeps Supervised under Write, so turning to Write never gives Full
// access by itself.
export function choiceOf(access: Access): AccessChoice {
  return access === 'read' ? { level: 'read', write: 'supervised' } : { level: 'write', write: access }
}
