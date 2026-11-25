import type { PR, Project, Session, State } from './types'

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

// goneThreads is the ids of the project's threads in before that its new list, after, no longer holds.
export function goneThreads(before: Session[], projectKey: string, after: Session[]): string[] {
  const kept = new Set(after.map((s) => s.id))
  return before.filter((s) => s.project === projectKey && !kept.has(s.id)).map((s) => s.id)
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

export type TaskTone = 'pending' | 'passing' | 'failing' | 'merged' | 'closed'
export type CurrentTask = { kind: 'pr'; number: number; tone: TaskTone } | { kind: 'issue'; number: number } | null

function prTone({ state, checks }: PR): TaskTone {
  if (state === 'merged' || state === 'closed') return state
  if (checks === 'passing' || checks === 'failing') return checks
  return 'pending'
}

// What the thread works towards, first match wins: its PR, toned by its checks, else its issue.
export function currentTask({ pr, issue }: Session): CurrentTask {
  if (pr) return { kind: 'pr', number: pr.number, tone: prTone(pr) }
  if (issue > 0) return { kind: 'issue', number: issue }
  return null
}

// The time since at as the thread card shows it: now, 2m, 4h, 1d, 3w; '' for no time.
export function shortAgo(at: number, now: number): string {
  if (!at) return ''
  const minutes = Math.floor(Math.max(0, now - at) / 60_000)
  if (minutes < 1) return 'now'
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  return days < 7 ? `${days}d` : `${Math.floor(days / 7)}w`
}

// What the thread's clean-up waits on, in words; '' when nothing does.
export function cleanupNote({ cleanup, cleanupReason, pr }: Session): string {
  if (cleanup === 'blocked') return `Not cleaned up: ${cleanupReason}`
  if (cleanup === 'pending') return 'Cleaning up…'
  if (cleanup === 'ask') return pr?.state === 'merged' ? 'PR was already merged' : 'PR closed without merging'
  return ''
}
