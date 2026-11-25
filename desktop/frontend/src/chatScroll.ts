import type { VirtualItem } from '@tanstack/react-virtual'

// isAtBottom says whether a scrolled view shows the end of its content, give or take slack px.
export function isAtBottom(top: number, client: number, height: number, slack = 48): boolean {
  return height - top - client <= slack
}

// The chat is at its end, and follows it, within END_SLACK px of it.
export const END_SLACK = 40

// Within ANCHOR_SLACK px of the end, the virtualizer keeps a growing chat that far from its end in the same frame. It is
// smaller than END_SLACK so that the last line stays clear of the composer: the gap above the composer is 12px at its
// narrowest and the last row ends in 20px of padding.
export const ANCHOR_SLACK = 32

// followScroll says whether a following chat scrolls to its end after it renders: when the composer over its end grew,
// which is no row the virtualizer anchors, or when the chat grew while too far from its end to be anchored.
export function followScroll({ following, grew, insetGrew, fromEnd }: { following: boolean; grew: boolean; insetGrew: boolean; fromEnd: number }): 'end' | 'stay' {
  if (!following) return 'stay'
  return insetGrew || (grew && fromEnd > ANCHOR_SLACK) ? 'end' : 'stay'
}

// KeptScroll is where a thread's chat was scrolled to and the sizes of the rows it measured, so that it opens there
// again before its rows are measured anew.
export type KeptScroll = { top: number; measured: VirtualItem[] }

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
