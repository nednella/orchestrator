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
