import type { PR, Project, Thread, State } from './types'

export type ThreadSections = { waiting: Thread[]; working: Thread[]; idle: Thread[]; settled: Thread[] }

function splitThreads(threads: Thread[]): ThreadSections {
  const of = (state: State) => threads.filter((s) => s.state === state)
  return { waiting: of('waiting'), working: of('working'), idle: of('idle'), settled: of('settled').sort((a, b) => b.lastEventAt - a.lastEventAt) }
}

export function matchesThread(thread: Thread, query: string): boolean {
  const q = query.trim().toLowerCase()
  if (!q) return true
  const number = q.replace(/^#/, '')
  return thread.title.toLowerCase().includes(q) || thread.branch.toLowerCase().includes(q) || (number !== '' && thread.issue > 0 && String(thread.issue).includes(number))
}

// isTopLevel is whether the thread shows in the sidebar: the owner started it, or the thread that delegated to it is
// no longer in the list, so a lost child shows rather than vanishes.
export function isTopLevel(s: Thread, byId: Map<string, Thread>): boolean {
  return s.parent === '' || !byId.has(s.parent)
}

function childrenOf(threads: Thread[]): Map<string, Thread[]> {
  const children = new Map<string, Thread[]>()
  for (const s of threads) {
    if (!s.parent) continue
    const list = children.get(s.parent)
    if (list) list.push(s)
    else children.set(s.parent, [s])
  }
  return children
}

// The thread's descendants, nearest first; seen keeps a loop in the parent links from walking forever.
function descendantsIn(id: string, children: Map<string, Thread[]>): Thread[] {
  const found: Thread[] = []
  const seen = new Set([id])
  for (let i = -1; i < found.length; i++) {
    for (const child of children.get(i < 0 ? id : found[i].id) ?? []) {
      if (seen.has(child.id)) continue
      seen.add(child.id)
      found.push(child)
    }
  }
  return found
}

export function descendantsOf(id: string, threads: Thread[]): Thread[] {
  return descendantsIn(id, childrenOf(threads))
}

const nearestWaiting = (s: Thread, descendants: Thread[]) => (s.state === 'waiting' ? s : descendants.find((d) => d.state === 'waiting'))

// ThreadTree is what the sidebar needs of the threads' families, built once per list: the top-level thread of every
// thread, itself for a top-level one, and for each top-level thread that needs the owner, the thread that waits.
export type ThreadTree = { top: Map<string, Thread>; attention: Map<string, Thread> }

export function threadTreeOf(threads: Thread[]): ThreadTree {
  const byId = new Map(threads.map((s) => [s.id, s]))
  const children = childrenOf(threads)
  const top = new Map<string, Thread>()
  const attention = new Map<string, Thread>()
  for (const s of threads) {
    if (!isTopLevel(s, byId)) continue
    const descendants = descendantsIn(s.id, children)
    top.set(s.id, s)
    descendants.forEach((d) => top.set(d.id, s))
    const waiting = nearestWaiting(s, descendants)
    if (waiting) attention.set(s.id, waiting)
  }
  return { top, attention }
}

// When the stretch of work the card shows began, or null when the card does not show work: a thread that waits, or
// has a waiting descendant (needs), is not at work, and one the app has no start for keeps the card's age.
export function workingSinceOf(thread: Thread, needs: Thread | undefined): number | null {
  return !needs && thread.state === 'working' && thread.workingSince > 0 ? thread.workingSince : null
}

// The thread list's order, of the top-level threads among threads: attention (a thread that waits or has a waiting
// descendant), working, idle, then settled, newest first.
export function sortThreads(threads: Thread[], tree = threadTreeOf(threads)): Thread[] {
  const shown = threads.filter((s) => tree.top.get(s.id) === s)
  const { working, idle, settled } = splitThreads(shown.filter((s) => !tree.attention.has(s.id)))
  return [...shown.filter((s) => tree.attention.has(s.id)), ...working, ...idle, ...settled]
}

export function sidebarThreads(threads: Thread[], projectKey: string | undefined, tree = threadTreeOf(threads)): Thread[] {
  return sortThreads(threads.filter((s) => s.project === projectKey), tree)
}

// The thread to open first among a project's list: its first top-level thread, in the list's order.
export function firstThread(list: Thread[], tree: ThreadTree): Thread | undefined {
  return list.find((s) => tree.top.get(s.id) === s)
}

// The thread to open when the one open, leaving, goes to sleep: the project's next live thread in sidebar order.
export function nextThread(threads: Thread[], projectKey: string | undefined, leaving: string, tree = threadTreeOf(threads)): Thread | undefined {
  return sidebarThreads(threads, projectKey, tree).find((s) => s.id !== leaving && s.state !== 'settled')
}

// goneThreads is the ids of the project's threads in before that its new list, after, no longer holds.
// splitSideChats parts a list of rows into the threads and the side chats, which show only in their threads' panels.
export function splitSideChats(rows: Thread[]): { threads: Thread[]; sideChats: Thread[] } {
  return { threads: rows.filter((s) => !s.side), sideChats: rows.filter((s) => s.side) }
}

export function goneThreads(before: Thread[], projectKey: string, after: Thread[]): string[] {
  const kept = new Set(after.map((s) => s.id))
  return before.filter((s) => s.project === projectKey && !kept.has(s.id)).map((s) => s.id)
}

// The threads that wait in each project, a child counted in its top-level thread's project, where its row shows it.
export function waitingByProject(threads: Thread[], tree = threadTreeOf(threads)): Map<string, number> {
  const counts = new Map<string, number>()
  for (const s of threads) {
    if (s.state !== 'waiting') continue
    const project = (tree.top.get(s.id) ?? s).project
    counts.set(project, (counts.get(project) ?? 0) + 1)
  }
  return counts
}

// The top-level threads the owner's next move is on, in the order the next-attention step visits them: those that
// need the owner, project by project, then those whose reply is in.
export function urgentThreads(projects: Project[], threads: Thread[], tree = threadTreeOf(threads)): Thread[] {
  const shown = projects.flatMap((p) => sidebarThreads(threads, p.key, tree))
  return [...shown.filter((s) => tree.attention.has(s.id)), ...shown.filter((s) => s.state === 'idle' && !tree.attention.has(s.id))]
}

export type GoalTone = 'pending' | 'passing' | 'failing' | 'merged' | 'closed'
export type Goal = { kind: 'pr'; number: number; tone: GoalTone } | { kind: 'issue'; number: number } | null

function prTone({ state, checks }: PR): GoalTone {
  if (state === 'merged' || state === 'closed') return state
  if (checks === 'passing' || checks === 'failing') return checks
  return 'pending'
}

// What the thread works towards, first match wins: its PR, toned by its checks, else its issue.
export function threadGoal({ pr, issue }: Thread): Goal {
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
export function cleanupNote({ cleanup, cleanupReason, pr }: Thread): string {
  if (cleanup === 'blocked') return `Kept: ${cleanupReason}`
  if (cleanup === 'pending') return 'Cleaning up…'
  if (cleanup === 'ask') return pr?.state === 'merged' ? 'PR was already merged' : 'PR closed without merging'
  return ''
}
