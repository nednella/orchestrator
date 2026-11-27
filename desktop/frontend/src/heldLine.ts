import type { Timeline } from './chatTimeline.ts'
import type { Thread } from './types.ts'

// withHeldError adds why the messages held for a running thread were not sent to the end of its chat: its start has
// ended, and they wait for the owner's next message.
export function withHeldError(timeline: Timeline, thread: Pick<Thread, 'start' | 'startError' | 'lastEventAt'>): Timeline {
  if (thread.start !== '' || thread.startError === '') return timeline
  const item = { kind: 'notice' as const, key: 'held-error', at: thread.lastEventAt, text: `Held messages not sent: ${thread.startError}`, error: true }
  return { ...timeline, items: [...timeline.items, item] }
}
