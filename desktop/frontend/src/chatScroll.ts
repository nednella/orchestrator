import type { VirtualItem } from '@tanstack/react-virtual'

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
