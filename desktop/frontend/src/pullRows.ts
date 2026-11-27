import type { RepoPull, Thread } from './types'

// pullRows splits the repository's open pull requests into the open thread's own and the rest. With no thread, or
// one whose pull request is not among them, nothing is the thread's.
export function pullRows(pulls: RepoPull[], thread: Thread | null): { mine: RepoPull | null; rest: RepoPull[] } {
  const mine = thread ? (pulls.find((p) => p.thread === thread.id) ?? null) : null
  return { mine, rest: mine ? pulls.filter((p) => p !== mine) : pulls }
}

export type PullsRead = { project: string; pulls: RepoPull[]; error: string }

// pullsShown is what the view shows for project: its own latest read, else the list known from before, else nothing yet.
export function pullsShown(read: PullsRead | null, project: string, known: Map<string, RepoPull[]>): { pulls: RepoPull[] | null; error: string } {
  if (read?.project === project) return { pulls: read.pulls, error: read.error }
  return { pulls: known.get(project) ?? null, error: '' }
}
