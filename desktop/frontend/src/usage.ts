import type { Limits } from './types'

export type LimitTone = 'plain' | 'warning' | 'danger'

// LimitRow is a plan window as the Usage modal shows it: the whole percent left and how long until it resets, '' when
// Claude does not say or the time has passed.
export type LimitRow = { label: string; left: number; resetsIn: string; tone: LimitTone }

// usageRows are the limits' windows at now, unix ms, in the order Go gives them. A window with 20% or less left warns,
// one with 10% or less is in danger; the tone follows the percent shown.
export function usageRows(l: Limits, now: number): LimitRow[] {
  return l.windows.map((w) => {
    const left = Math.round(Math.min(100, Math.max(0, w.left)))
    const tone: LimitTone = left <= 10 ? 'danger' : left <= 20 ? 'warning' : 'plain'
    return { label: w.label, left, resetsIn: w.resetsAt > 0 ? resetsIn(w.resetsAt - now) : '', tone }
  })
}

// resetsIn is a span of ms in its two largest units, dropping a zero second one: "45m", "2h 5m", "3d 4h", "2d". A span
// under a minute is "1m"; one that is over is ''.
export function resetsIn(ms: number): string {
  if (ms <= 0) return ''
  const minutes = Math.max(1, Math.floor(ms / 60_000))
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return minutes % 60 ? `${hours}h ${minutes % 60}m` : `${hours}h`
  const days = Math.floor(hours / 24)
  return hours % 24 ? `${days}d ${hours % 24}h` : `${days}d`
}
