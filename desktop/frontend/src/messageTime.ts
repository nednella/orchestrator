const formats = new Map<string, Intl.DateTimeFormat>()

function format(locale: string | undefined, options: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  const key = `${locale ?? ''}|${JSON.stringify(options)}`
  let f = formats.get(key)
  if (!f) {
    f = new Intl.DateTimeFormat(locale, options)
    formats.set(key, f)
  }
  return f
}

const clock: Intl.DateTimeFormatOptions = { hour: 'numeric', minute: '2-digit' }

const dayOf = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()

// messageTime is when a message was sent, as its row shows it: the time on the day it was sent, "yesterday at" the time
// the day after, and the day and month before the time later on, with the year once it is another year.
export function messageTime(at: number, now: number, locale?: string): string {
  const sent = new Date(at)
  const today = new Date(now)
  const time = format(locale, clock).format(sent)
  if (dayOf(sent) === dayOf(today)) return time
  const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1)
  if (dayOf(sent) === yesterday.getTime()) return `yesterday at ${time}`
  const year: Intl.DateTimeFormatOptions = sent.getFullYear() === today.getFullYear() ? {} : { year: 'numeric' }
  return format(locale, { day: 'numeric', month: 'numeric', ...year, ...clock }).format(sent)
}

const ordinals = new Intl.PluralRules('en', { type: 'ordinal' })
const suffixes: Record<string, string> = { one: 'st', two: 'nd', few: 'rd', other: 'th' }

// messageTimeTitle is the long form of the time, as "14:54, 25th November 2026".
export function messageTimeTitle(at: number, locale?: string): string {
  const sent = new Date(at)
  const day = sent.getDate()
  const month = format(locale, { month: 'long' }).format(sent)
  return `${format(locale, clock).format(sent)}, ${day}${suffixes[ordinals.select(day)]} ${month} ${sent.getFullYear()}`
}
