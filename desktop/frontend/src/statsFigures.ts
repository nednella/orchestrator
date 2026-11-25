import type { WaitKind, WorkRow } from './types'

export type Change = { direction: 'up' | 'down' | 'flat'; text: string }

// The change on the period before, in whole percent; null when the period before had nothing to compare with.
export function change(now: number, before: number): Change | null {
  if (before <= 0) return null
  const percent = Math.round(((now - before) / before) * 100)
  if (percent === 0) return { direction: 'flat', text: '0%' }
  return { direction: percent > 0 ? 'up' : 'down', text: `${Math.abs(percent).toLocaleString('en-GB')}%` }
}

// The period a range is compared with, '' for lifetime.
export function periodBefore(days: number): string {
  if (days === 0) return ''
  if (days === 1) return 'yesterday'
  return `the ${days} days before`
}

export function changeLabel(c: Change, period: string): string {
  if (c.direction === 'flat') return `No change on ${period}`
  return `${c.direction === 'up' ? 'Up' : 'Down'} ${c.text} on ${period}`
}

// Time as hours and minutes, never days: "61h 20m"; past 100 hours the minutes no longer tell much. Under an
// hour, seconds adds them: "1m 12s".
export function workTime(ms: number, seconds = false): string {
  const secs = Math.floor(Math.max(0, ms) / 1000)
  const minutes = Math.floor(secs / 60)
  if (minutes < 1) return secs ? `${secs}s` : '0m'
  if (minutes < 60) return seconds && secs % 60 ? `${minutes}m ${secs % 60}s` : `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  if (hours >= 100 || rest === 0) return `${hours.toLocaleString('en-GB')}h`
  return `${hours}h ${rest}m`
}

// A count with its noun: "1 issue", "54 issues".
export function counted(n: number, one: string, many: string): string {
  return `${n.toLocaleString('en-GB')} ${n === 1 ? one : many}`
}

// How many a day: one decimal under ten, whole above.
export function aDay(total: number, days: number): string {
  const n = total / Math.max(days, 1)
  return n >= 10 ? Math.round(n).toLocaleString('en-GB') : String(Number(n.toFixed(1)))
}

// A chart's axis value of worked time, in hours, or minutes when the top is under two hours.
export function workAxis(ms: number, top: number): string {
  return top >= 7_200_000 ? `${Math.round(ms / 3_600_000)}h` : `${Math.round(ms / 60_000)}m`
}

// A chart's axis value of cost, in whole dollars, or cents when the top is under ten dollars.
export function costAxis(cost: number, top: number): string {
  return top >= 10 ? `$${Math.round(cost).toLocaleString('en-GB')}` : `$${cost.toFixed(2)}`
}

// The issue number that heads a work row's title, apart from the rest of it.
export function workTitle(row: Pick<WorkRow, 'issue' | 'title'>): { ref: string; title: string } {
  if (row.issue === 0) return { ref: '', title: row.title }
  const ref = `#${row.issue}`
  return { ref, title: row.title.startsWith(`${ref} `) ? row.title.slice(ref.length + 1) : row.title }
}

// The waits of each kind, most first.
export function kindCounts(byCause: { kind: WaitKind; count: number }[]): { kind: WaitKind; count: number }[] {
  const counts = new Map<WaitKind, number>()
  for (const c of byCause) counts.set(c.kind, (counts.get(c.kind) ?? 0) + c.count)
  return [...counts].map(([kind, count]) => ({ kind, count })).sort((a, b) => b.count - a.count)
}

