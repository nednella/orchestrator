import type { Access } from './types'

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

// showsAccessIcon says whether the model chip marks the access: only one other than the default access setting.
export function showsAccessIcon(access: Access, defaultAccess: Access | ''): boolean {
  return access !== newThreadAccess(defaultAccess)
}

export const ACCESS_HINT: Record<Access, string> = {
  read: 'It reads the project and plans; Claude asks before it changes anything',
  supervised: 'Your Claude allow rules apply; other tools ask in the chat',
  full: 'Every tool runs without asking',
}

// accessDisabled is why a thread's access cannot change now, "" when it can: a turn keeps the access it started with,
// and a change restarts Claude, which would stop the subagents that run.
export function accessDisabled(turnOpen: boolean, pending: boolean, agentsRunning = false): string {
  if (turnOpen || pending) return 'Change access when the turn ends'
  return agentsRunning ? 'Change access when the subagents end' : ''
}
