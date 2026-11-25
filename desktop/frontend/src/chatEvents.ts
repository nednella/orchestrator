import type { ChatEvent } from './types'

// ChatState is a thread's chat as the front end holds it: its logged events in seq order, the seq of the last one,
// and the text streamed so far of each message, by message id, that is not logged yet.
export type ChatState = { events: ChatEvent[]; last: number; live: Record<string, string> }

export const EMPTY_CHAT: ChatState = { events: [], last: 0, live: {} }

// mergeEvents adds events from the stream or the history. A logged event the state already holds is dropped; one past
// a gap is dropped too, and gapAfter says to read the history after it, which brings it again.
export function mergeEvents(s: ChatState, incoming: ChatEvent[]): { state: ChatState; gapAfter: number | null } {
  let events = s.events
  const live = { ...s.live }
  let last = s.last
  let gapAfter: number | null = null
  let changed = false
  for (const e of incoming) {
    if (e.kind === 'delta') {
      if (!e.id || !e.text) continue
      live[e.id] = (live[e.id] ?? '') + e.text
      changed = true
      continue
    }
    // A double clears too: the stream can send a block's deltas, the block, then the next block's deltas, while the
    // block came first in the history read meanwhile.
    if (e.kind === 'message' && e.id && e.id in live) {
      delete live[e.id]
      changed = true
    }
    // A reply cut off by an interrupt or an exit streamed text that is never logged.
    if ((e.kind === 'turn_end' || e.kind === 'exit') && Object.keys(live).length > 0) {
      for (const id of Object.keys(live)) delete live[id]
      changed = true
    }
    if (e.seq <= last) continue
    if (e.seq > last + 1) {
      gapAfter ??= last
      continue
    }
    if (events === s.events) events = [...s.events]
    events.push(e)
    last = e.seq
    changed = true
  }
  return { state: changed ? { events, last, live } : s, gapAfter }
}

// Events that come before the first history read are held for it; past this many, they are dropped and read again.
const heldBeforeLoad = 2000

export type ChatFeedDeps = {
  history(after: number): Promise<ChatEvent[]>
  // later runs run soon, once, and gives back a way to cancel it.
  later(run: () => void): () => void
  onState(state: ChatState): void
  onError(err: unknown): void
}

// chatFeed keeps a thread's state from its history and its live events. Live events are held until the first history
// read, which goes first, then merged in batches, in the order they came: a delta merged after its message would show
// its text again. A gap is filled from the history; one seen while a fill runs is filled once that fill ends.
export function chatFeed(start: ChatState, deps: ChatFeedDeps) {
  let state = start
  let held: ChatEvent[] = []
  let loaded = false
  let dropped = false
  let stopped = false
  let filling = false
  let gapWhileFilling = false
  let cancel: (() => void) | null = null

  const apply = (incoming: ChatEvent[]) => {
    const { state: next, gapAfter } = mergeEvents(state, incoming)
    if (next !== state) {
      state = next
      deps.onState(next)
    }
    if (gapAfter !== null) fill(gapAfter)
  }
  const fill = (after: number) => {
    if (filling) {
      gapWhileFilling = true
      return
    }
    filling = true
    deps
      .history(after)
      .then(
        (events) => {
          if (!stopped) apply(events)
        },
        (err) => {
          gapWhileFilling = false
          if (!stopped) deps.onError(err)
        },
      )
      .finally(() => {
        filling = false
        if (!gapWhileFilling || stopped) return
        gapWhileFilling = false
        fill(state.last)
      })
  }
  const flush = () => {
    cancel = null
    const events = held
    held = []
    apply(events)
  }

  return {
    push(events: ChatEvent[]) {
      if (stopped) return
      held.push(...events)
      if (loaded) {
        cancel ??= deps.later(flush)
        return
      }
      if (held.length <= heldBeforeLoad) return
      held = []
      dropped = true
    },
    // load reads the history after the state's last seq. When it fails, the feed stops.
    load(): Promise<void> {
      return deps.history(state.last).then(
        (events) => {
          if (stopped) return
          loaded = true
          held = [...events, ...held]
          flush()
          if (dropped) fill(state.last)
        },
        (err) => {
          if (stopped) return
          stopped = true
          held = []
          deps.onError(err)
        },
      )
    },
    stop() {
      stopped = true
      held = []
      cancel?.()
    },
  }
}
