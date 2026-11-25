import type { Digest } from './types'

export type RunState = 'running' | 'failed' | 'never' | 'checked'

type RunTimes = Pick<Digest, 'running' | 'startedAt' | 'lastRunAt' | 'nextRunAt' | 'error' | 'stoppedAt'>

// A start after the last finish with no error is a run that failed before a restart, which forgets why.
export function runState(d: RunTimes): RunState {
  if (d.running) return 'running'
  if (d.error || d.startedAt > d.lastRunAt) return 'failed'
  return d.lastRunAt === 0 ? 'never' : 'checked'
}

const minute = 60_000
const hour = 60 * minute
const day = 24 * hour

const count = (n: number, unit: string) => `${n} ${unit}${n === 1 ? '' : 's'}`

export function agoWords(at: number, now: number): string {
  const ms = Math.max(0, now - at)
  if (ms < minute) return 'just now'
  if (ms < hour) return `${count(Math.floor(ms / minute), 'minute')} ago`
  if (ms < day) return `${count(Math.floor(ms / hour), 'hour')} ago`
  return `${count(Math.floor(ms / day), 'day')} ago`
}

// took writes how long a run took, as the thread's "Worked for" does.
export function runStatus(d: RunTimes, now: number, took: (ms: number) => string): string {
  switch (runState(d)) {
    case 'running':
      return `Checking for ${took(now - d.startedAt)}`
    case 'failed':
      return d.stoppedAt ? `Stopped after ${took(d.stoppedAt - d.startedAt)}, ${agoWords(d.stoppedAt, now)}` : 'Last run failed'
    case 'never':
      return 'Never run'
    case 'checked': {
      const ago = `Checked ${agoWords(d.lastRunAt, now)}`
      return d.startedAt > 0 ? `${ago} in ${took(d.lastRunAt - d.startedAt)}` : ago
    }
  }
}

// What the status row shows at its end: when the run started, or when the next one is due.
export function runStatusEnd(d: RunTimes, now: number): string {
  if (d.running) return `Started ${new Date(d.startedAt).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })}`
  if (d.nextRunAt === 0) return 'Automatic runs off'
  if (d.nextRunAt <= now) return 'Next run soon'
  return `Next run ${new Date(d.nextRunAt).toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' })}`
}
