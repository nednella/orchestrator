import type { AgentItem } from './chatTimeline'

const twoDigits = (n: number) => String(n).padStart(2, '0')

// elapsed is a span as a subagent's status says it: 12s, 1m 02s, 1h 02m.
function elapsed(ms: number): string {
  const seconds = Math.max(0, Math.floor(ms / 1000))
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${twoDigits(seconds % 60)}s`
  return `${Math.floor(seconds / 3600)}h ${twoDigits(Math.floor(seconds / 60) % 60)}m`
}

// subagentStatus is where the subagent is at now: starting until its first step, then running for how long so far,
// then how it ended.
export function subagentStatus(a: AgentItem, now: number): string {
  switch (a.state) {
    case 'running':
      return a.steps.length === 0 ? 'Starting' : `Running ${elapsed(now - a.startedAt)}`
    case 'done':
      return `Completed in ${elapsed((a.endedAt ?? now) - a.startedAt)}`
    case 'failed':
      return 'Failed'
    case 'stopped':
      return 'Stopped'
  }
}

// AgentView is the subagent whose view shows in place of its thread's chat.
export type AgentView = { thread: string; agent: string }

// viewedIn is the subagent of the thread whose view is open, null while the thread shows its chat.
export const viewedIn = (view: AgentView | null, thread: string): string | null => (view?.thread === thread ? view.agent : null)

// closedIn is the view once the thread goes back to its chat: another thread's view is left as it is.
export const closedIn = (view: AgentView | null, thread: string): AgentView | null => (view?.thread === thread ? null : view)

// keptFor is the view while the thread is on screen: a view of any other thread closes, so no thread opens on a
// subagent the owner did not open while it showed.
export const keptFor = (view: AgentView | null, selected: string): AgentView | null => (view?.thread === selected ? view : null)

// heldBy is the view once the thread's chat has loaded: it closes when the chat does not hold the view's subagent.
export const heldBy = (view: AgentView | null, thread: string, loaded: boolean, holds: boolean): AgentView | null => (loaded && !holds && view?.thread === thread ? null : view)
