import type { VirtualItem } from '@tanstack/react-virtual'
import type { TimelineItem } from './chatTimeline'

// isAtBottom says whether a scrolled view shows the end of its content, give or take slack px.
export function isAtBottom(top: number, client: number, height: number, slack = 48): boolean {
  return height - top - client <= slack
}

// followScroll says whether the chat scrolls to its end as it grows: only when the reader was at the end.
export function followScroll({ atBottom, grew }: { atBottom: boolean; grew: boolean }): 'end' | 'stay' {
  return atBottom && grew ? 'end' : 'stay'
}

// KeptScroll is where a thread's chat was scrolled to, the sizes of the rows it measured, so that it opens there again
// before its rows are measured anew, and how many items it had when the reader last reached its end.
export type KeptScroll = { top: number; measured: VirtualItem[]; count: number }

const keptThreads = 20
const kept = new Map<string, KeptScroll>()

// keepScroll keeps where a thread's chat was left. null is the bottom, where every thread opens.
export function keepScroll(id: string, at: KeptScroll | null) {
  kept.delete(id)
  if (!at) return
  kept.set(id, at)
  const oldest = kept.keys().next().value
  if (kept.size > keptThreads && oldest !== undefined) kept.delete(oldest)
}

export function keptScroll(id: string): KeptScroll | null {
  return kept.get(id) ?? null
}

// hasUnseen says whether a thread's chat, opened where it was left, has items the reader has not seen: more than it
// had when the reader last reached its end.
export function hasUnseen(kept: KeptScroll | null, count: number): boolean {
  return kept !== null && count > kept.count
}

export type MinimapMark = { top: number; kind: 'user' | 'approval' | 'error'; key: string }

const markPriority: Record<MinimapMark['kind'], number> = { user: 0, error: 1, approval: 2 }
const markGap = 3

const markKind = (item: TimelineItem): MinimapMark['kind'] | null => (item.kind === 'user' || item.kind === 'approval' ? item.kind : item.kind === 'notice' && item.error ? 'error' : null)

// minimapMarks places the items a reader looks for on a bar of the given height: their own messages, approvals that
// wait and errors. starts holds each item's offset in the chat, total the chat's height. Marks closer than 3 px merge
// into the one of the highest priority.
export function minimapMarks(items: TimelineItem[], starts: number[], total: number, height: number): MinimapMark[] {
  if (total <= 0) return []
  const marks: MinimapMark[] = []
  let anchor = 0
  items.forEach((item, i) => {
    const kind = markKind(item)
    if (!kind || starts[i] === undefined) return
    const mark = { top: (starts[i] / total) * height, kind, key: item.key }
    const last = marks[marks.length - 1]
    if (!last || mark.top - anchor >= markGap) {
      marks.push(mark)
      anchor = mark.top
    }
    else if (markPriority[kind] > markPriority[last.kind]) marks[marks.length - 1] = mark
  })
  return marks
}

const minThumb = 24

// thumb sizes and places the bar's handle for the part of the chat in view.
export function thumb(scrollTop: number, client: number, total: number, height: number): { top: number; size: number } {
  if (total <= client) return { top: 0, size: height }
  const size = Math.min(height, Math.max(minThumb, (client / total) * height))
  return { top: (scrollTop / (total - client)) * (height - size), size }
}

// trackToScroll gives the scroll offset that puts the chat's point at y on the bar in the middle of the view.
export function trackToScroll(y: number, height: number, total: number, client: number): number {
  return Math.min(Math.max((y / height) * total - client / 2, 0), Math.max(total - client, 0))
}
