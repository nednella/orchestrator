import type { Timeline } from './chatTimeline'
import type { ChatEvent } from './types'

export type EscapeAction = 'close-overlay' | 'close-picker' | 'interrupt' | 'blur' | 'none'
export type ComposerAction = 'send' | 'queue' | 'deny' | 'stop'

// escapeAction is what Esc does in a thread's chat: it closes what is open over the chat first, and stops a running
// turn only once.
export function escapeAction({ overlay, picker, turnOpen, stopping }: { overlay: boolean; picker: boolean; turnOpen: boolean; stopping: boolean }): EscapeAction {
  if (overlay) return 'close-overlay'
  if (picker) return 'close-picker'
  if (!turnOpen) return 'blur'
  return stopping ? 'none' : 'interrupt'
}

// composerAction is what the composer's button does: with the turn open and nothing typed it stops the turn, and a
// message denies the approval that waits, or is queued while the turn runs.
export function composerAction({ turnOpen, pending }: Pick<Timeline, 'turnOpen' | 'pending'>, draftEmpty: boolean): ComposerAction {
  if (turnOpen && draftEmpty) return 'stop'
  if (pending) return 'deny'
  return turnOpen ? 'queue' : 'send'
}

// unseenDrafts is the drafts agentos left in the thread's composer that it has not shown yet: those after the seen
// mark and after the last message the user sent, which took in any draft before it.
export function unseenDrafts(events: ChatEvent[], seenSeq: number): ChatEvent[] {
  const after = Math.max(seenSeq, events.findLast((e) => e.kind === 'user')?.seq ?? 0)
  return events.filter((e) => e.kind === 'draft' && e.seq > after)
}

// sendText is the message the composer's markdown sends. A leading indent can be code, so only the end is trimmed.
export const sendText = (markdown: string) => markdown.replace(/\s+$/, '')

// lastTurnEnd is the seq of the latest event that ended a turn: a stop asked for after it waits for the next one.
export const lastTurnEnd = (events: ChatEvent[]) => events.findLast((e) => e.kind === 'turn_end' || e.kind === 'exit')?.seq ?? 0
