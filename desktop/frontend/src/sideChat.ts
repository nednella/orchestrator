import type { ChatEvent, Session, State } from './types'

export const SIDE_CHAT_TITLE = 'Side chat'

// ownStart is where a side chat's own chat starts in its events: at its host's first start, since the chat it was
// forked with is its thread's. Before that start all of them are the thread's.
export function ownStart(events: ChatEvent[]): number {
  const at = events.findIndex((e) => e.kind === 'start')
  return at < 0 ? events.length : at
}

// sideChatAsked says whether the side chat was sent a message of its own.
export const sideChatAsked = (events: ChatEvent[]): boolean => events.slice(ownStart(events)).some((e) => e.kind === 'user' && !e.parent)

// forkableReply says whether a side chat can fork the chat now, as the sessions find the place: a reply of the
// thread's own that a turn's end follows, not taken back by a rewind, and from after the last compaction.
function forkableReply(events: ChatEvent[]): 'yes' | 'none' | 'compacted' {
  let ended = false
  for (let i = events.length - 1; i >= 0; i--) {
    const e = events[i]
    if (e.kind === 'compact') return events.slice(0, i).some((m) => m.kind === 'message' && m.uuid) ? 'compacted' : 'none'
    if (e.kind === 'rewind') {
      const from = events.slice(0, i).findIndex((u) => u.kind === 'user' && u.id === e.id)
      if (from >= 0) i = from
    } else if (e.kind === 'turn_end') ended = true
    else if (ended && e.kind === 'message' && e.uuid && !e.agent) return 'yes'
  }
  return 'none'
}

// sideChatReason is why the thread cannot start a side chat, '' when it can as far as the app knows. events is its
// chat once loaded, undefined before: the sessions refuse what the app cannot tell.
export function sideChatReason(s: Session | undefined, events?: ChatEvent[]): string {
  if (!s) return 'Available from a thread.'
  if (s.side) return 'Side chats cannot start side chats.'
  if (s.parent) return 'Subagent threads cannot start side chats.'
  if (!events) return ''
  const reply = forkableReply(events)
  if (reply === 'compacted') return "This thread's last reply is from before its chat was compacted. Side chats start after its next reply."
  return reply === 'none' ? 'Side chats start after the first message.' : ''
}

// The dot on a side chat's tab: grey while it works, the info colour while it needs the owner, none otherwise.
export function sideChatDot(state: State): 'busy' | 'asks' | null {
  if (state === 'working') return 'busy'
  return state === 'waiting' ? 'asks' : null
}
