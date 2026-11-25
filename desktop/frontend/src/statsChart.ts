import type { DayFigures } from './types'

export type Metric = 'prompts' | 'worked' | 'cost'

// One bar of the chart: a day, or a week from Monday when the range is long.
export type Bar = { day: string; label: string; value: number; tick: string }

// A range of this many days or more shows a bar a week: a bar a day would be thinner than a pixel or two.
const WEEKLY_FROM = 93

// Ticks stay at least this far apart, counted in a fourteenth of the bars.
const TICKS = 14

const date = (day: string) => {
  const [y, m, d] = day.split('-').map(Number)
  return new Date(y, m - 1, d)
}

export const dayName = (day: string) => date(day).toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' })

export const shortDay = (day: string) => date(day).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })

// A month's tick; January's is its year, which also keeps it short.
const monthTick = (day: string) => {
  const d = date(day)
  return d.getMonth() === 0 ? String(d.getFullYear()) : d.toLocaleDateString('en-GB', { month: 'short' })
}

export function valueOf(d: DayFigures, metric: Metric): number {
  if (metric === 'prompts') return d.prompts
  if (metric === 'worked') return d.workMs
  return d.cost
}

export const weekly = (days: DayFigures[]) => days.length >= WEEKLY_FROM

// The chart's bars, oldest first; the last one is today, or this week, so far.
export function chartBars(days: DayFigures[], metric: Metric): Bar[] {
  const byWeek = weekly(days)
  const bars: Bar[] = []
  for (const d of days) {
    const last = bars.at(-1)
    if (byWeek && last && date(d.day).getDay() !== 1) {
      last.value += valueOf(d, metric)
      continue
    }
    bars.push({ day: d.day, label: byWeek ? `Week of ${shortDay(d.day)}` : dayName(d.day), value: valueOf(d, metric), tick: '' })
  }
  const last = bars.at(-1)
  if (last) last.label += byWeek ? ' (this week, so far)' : ' (today, so far)'
  return withTicks(bars, byWeek)
}

// Days are ticked on Mondays, or each one when there are few; weeks where a month starts. A tick too close to
// the one before is left out.
function withTicks(bars: Bar[], byWeek: boolean): Bar[] {
  const gap = Math.ceil(bars.length / TICKS)
  let last = -gap
  return bars.map((bar, i) => {
    const d = date(bar.day)
    const due = byWeek ? i === 0 || date(bars[i - 1].day).getMonth() !== d.getMonth() : bars.length <= TICKS || d.getDay() === 1
    if (!due || i - last < gap) return bar
    last = i
    return { ...bar, tick: byWeek ? monthTick(bar.day) : shortDay(bar.day) }
  })
}

// Which bar had the most, as "most on Wed 11 Nov"; '' when every bar is empty. The first one wins a tie.
export function busiest(bars: Bar[], byWeek: boolean): string {
  const top = Math.max(0, ...bars.map((b) => b.value))
  const bar = bars.find((b) => b.value === top)
  if (top === 0 || !bar) return ''
  return byWeek ? `most in the week of ${shortDay(bar.day)}` : `most on ${dayName(bar.day)}`
}

// Where a bar's hover tip sits, so a tip at either end stays inside the chart.
export function tipAlign(i: number, count: number): 'start' | 'center' | 'end' {
  if (i < count * 0.15) return 'start'
  if (i >= count * 0.85) return 'end'
  return 'center'
}

// The range the days cover, as "27 Oct – 25 Nov", with years when they differ, or one day's name.
export function rangeLabel(first: string, last: string): string {
  if (first === last) return dayName(first)
  if (first.slice(0, 4) === last.slice(0, 4)) return `${shortDay(first)} – ${shortDay(last)}`
  const withYear = (day: string) => date(day).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
  return `${withYear(first)} – ${withYear(last)}`
}
