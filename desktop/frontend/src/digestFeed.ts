import type { DigestItem } from './types'

export type DigestFilter = 'all' | 'new' | 'saved'

export type DigestTool = { key: string; name: string; items: DigestItem[]; fresh: number }

// New is what the owner has not seen: not saved, and found after the page was last opened.
export const isNewItem = (item: DigestItem, seen: number) => !item.noteId && item.at > seen

const matches = (item: DigestItem, filter: DigestFilter, seen: number) => {
  if (filter === 'new') return isNewItem(item, seen)
  if (filter === 'saved') return item.noteId !== ''
  return true
}

export function filterCounts(items: DigestItem[], seen: number): Record<DigestFilter, number> {
  return {
    all: items.length,
    new: items.filter((i) => isNewItem(i, seen)).length,
    saved: items.filter((i) => i.noteId !== '').length,
  }
}

// The run names each item's tool in its own words, so one tool can come as "wails" and "Wails".
const toolKey = (source: string) => source.trim().toLowerCase() || 'other'

// One group per tool, in the digest's order: the latest run's items first, in the order the run found them.
export function groupByTool(items: DigestItem[], filter: DigestFilter, seen: number): DigestTool[] {
  const tools = new Map<string, DigestTool>()
  for (const item of items) {
    if (!matches(item, filter, seen)) continue
    const key = toolKey(item.source)
    const tool = tools.get(key) ?? { key, name: item.source.trim() || 'Other', items: [], fresh: 0 }
    tool.items.push(item)
    if (isNewItem(item, seen)) tool.fresh++
    tools.set(key, tool)
  }
  return [...tools.values()]
}

export function itemHost(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return ''
  }
}

// "23 Nov", with the year when it is not this year's.
export function itemDate(at: number, now: number): string {
  const sameYear = new Date(at).getFullYear() === new Date(now).getFullYear()
  return new Date(at).toLocaleDateString('en-GB', sameYear ? { day: 'numeric', month: 'short' } : { day: 'numeric', month: 'short', year: 'numeric' })
}
