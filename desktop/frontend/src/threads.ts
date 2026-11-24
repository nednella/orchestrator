import type { Project, Session, State } from './types'

export type ThreadSections = { waiting: Session[]; working: Session[]; idle: Session[]; settled: Session[] }

export function splitThreads(sessions: Session[]): ThreadSections {
  const of = (state: State) => sessions.filter((s) => s.state === state)
  return { waiting: of('waiting'), working: of('working'), idle: of('idle'), settled: of('ended').sort((a, b) => b.lastEventAt - a.lastEventAt) }
}

export function matchesThread(session: Session, query: string): boolean {
  const q = query.trim().toLowerCase()
  if (!q) return true
  const number = q.replace(/^#/, '')
  return session.title.toLowerCase().includes(q) || session.branch.toLowerCase().includes(q) || (number !== '' && session.issue > 0 && String(session.issue).includes(number))
}

// The thread list's order: needs you, working, idle, then ended, newest first.
export function sortThreads(sessions: Session[]): Session[] {
  const { waiting, working, idle, settled } = splitThreads(sessions)
  return [...waiting, ...working, ...idle, ...settled]
}

export function sidebarThreads(sessions: Session[], projectKey: string | undefined): Session[] {
  return sortThreads(sessions.filter((s) => s.project === projectKey))
}

export function waitingByProject(sessions: Session[]): Map<string, number> {
  const counts = new Map<string, number>()
  for (const s of sessions) if (s.state === 'waiting') counts.set(s.project, (counts.get(s.project) ?? 0) + 1)
  return counts
}

export function liveThreads(projects: Project[], sessions: Session[]): Session[] {
  return projects.flatMap((p) => {
    const { waiting, working, idle } = splitThreads(sessions.filter((s) => s.project === p.key))
    return [...waiting, ...working, ...idle]
  })
}

// What the thread's clean-up waits on, in words; '' when nothing does.
export function cleanupNote({ cleanup, cleanupReason, pr }: Session): string {
  if (cleanup === 'blocked') return `Not cleaned up: ${cleanupReason}`
  if (cleanup === 'pending') return 'Cleaning up…'
  if (cleanup === 'ask') return pr?.state === 'merged' ? 'PR was already merged' : 'PR closed without merging'
  return ''
}
