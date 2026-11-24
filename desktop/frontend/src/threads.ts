import type { Project, Session, State } from './types'

const LIVE_ORDER: State[] = ['waiting', 'working', 'idle']

export function splitThreads(sessions: Session[]): { live: Session[]; settled: Session[] } {
  const live = LIVE_ORDER.flatMap((state) => sessions.filter((s) => s.state === state))
  return { live, settled: sessions.filter((s) => s.state === 'ended') }
}

export function matchesThread(session: Session, query: string): boolean {
  const q = query.trim().toLowerCase()
  if (!q) return true
  const number = q.replace(/^#/, '')
  return session.title.toLowerCase().includes(q) || session.branch.toLowerCase().includes(q) || (number !== '' && session.issue > 0 && String(session.issue).includes(number))
}

export function sidebarThreads(projects: Project[], sessions: Session[]): { groups: { project: Project; sessions: Session[] }[]; settled: Session[] } {
  const { settled } = splitThreads(sessions)
  return {
    groups: projects.map((project) => ({ project, sessions: splitThreads(sessions.filter((s) => s.project === project.key)).live })).filter((g) => g.sessions.length > 0),
    settled: settled.sort((a, b) => b.lastEventAt - a.lastEventAt),
  }
}
