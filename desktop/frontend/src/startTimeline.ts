import { buildTimeline } from './chatTimeline.ts'
import type { Timeline, TimelineItem } from './chatTimeline.ts'
import type { ChatEvent, Thread } from './types.ts'

type Starting = Pick<Thread, 'id' | 'createdAt' | 'start' | 'startError' | 'detail' | 'firstMessage' | 'startQueue'>

// startTimeline is the chat of a thread whose agent has not started, as it will look once the agent has taken the
// message: the message sent, the turn running, and what the app sets up said quietly in the working row. Messages
// sent since wait queued. A start that failed ends the turn with the reason and a retry where the working row was.
export function startTimeline(thread: Starting): { timeline: Timeline; events: ChatEvent[] } {
  const events: ChatEvent[] = []
  if (thread.firstMessage) events.push({ seq: 1, at: thread.createdAt, kind: 'user', id: `${thread.id}/first`, text: thread.firstMessage })
  thread.startQueue.forEach((text, i) => events.push({ seq: 2 + i, at: thread.createdAt, kind: 'queued', id: `${thread.id}/queued-${i}`, text }))
  const built = buildTimeline(events, {}, thread.id)
  const items: TimelineItem[] = built.items.filter((item) => item.kind !== 'thinking')
  if (thread.start === 'failed') {
    items.push({ kind: 'startFailed', key: 'start-failed', text: thread.startError || 'The thread did not start' })
    return { timeline: { ...built, items, turnOpen: false }, events }
  }
  items.push({ kind: 'thinking', key: 'thinking-start', label: thread.detail || 'Starting' })
  return { timeline: { ...built, items, turnOpen: true }, events }
}
