import type { RepoPull, Session } from './types'

// pullRows splits the repository's open pull requests into the open thread's own and the rest. With no thread, or
// one whose pull request is not among them, nothing is the thread's.
export function pullRows(pulls: RepoPull[], thread: Session | null): { mine: RepoPull | null; rest: RepoPull[] } {
  const mine = thread ? (pulls.find((p) => p.thread === thread.id) ?? null) : null
  return { mine, rest: mine ? pulls.filter((p) => p !== mine) : pulls }
}
