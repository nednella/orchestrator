import { useEffect, useMemo, useState } from 'react'
import { api, chatEvent, errorMessage, on } from './api'
import { EMPTY_CHAT, mergeEvents } from './chatEvents'
import type { ChatState } from './chatEvents'
import { buildTimeline } from './chatTimeline'
import type { Timeline } from './chatTimeline'
import type { ChatEvent } from './types'

const keptThreads = 20
const kept = new Map<string, ChatState>()

function keep(id: string, state: ChatState) {
  kept.delete(id)
  kept.set(id, state)
  const oldest = kept.keys().next().value
  if (kept.size > keptThreads && oldest !== undefined) kept.delete(oldest)
}

// The text streamed while a thread was not watched is lost, so a kept thread starts again with none.
function startOf(id: string): View {
  const state = kept.get(id)
  return { id, state: state ? { ...state, live: {} } : EMPTY_CHAT, loading: !state, error: '' }
}

const calls = new Map<string, Promise<unknown>>()

// inTurn sends one thread's watch and unwatch one after another. Go serves each call on a goroutine of its own, so an
// unwatch sent just before a watch, as when the view leaves a thread and comes back, could land after it.
function inTurn<T>(id: string, call: () => Promise<T>): Promise<T> {
  const next = (calls.get(id) ?? Promise.resolve()).then(call, call)
  calls.set(id, next.catch(() => undefined))
  return next
}

type View = { id: string; state: ChatState; loading: boolean; error: string }

// useChat is the thread's chat: its history, then its live events, as a timeline.
export function useChat(id: string): { timeline: Timeline; loading: boolean; error: string } {
  const [view, setView] = useState(() => startOf(id))
  const shown = view.id === id ? view : startOf(id)
  const timeline = useMemo(() => buildTimeline(shown.state.events, shown.state.live, id), [shown.state, id])

  useEffect(() => {
    const start = startOf(id)
    setView(start)
    let state = start.state
    let held: ChatEvent[] = []
    let loaded = false
    let frame = 0
    let filling = false
    let gone = false

    const fail = (err: unknown) => {
      if (!gone) setView((v) => ({ ...v, loading: false, error: errorMessage(err) }))
    }
    const apply = (incoming: ChatEvent[]) => {
      const { state: next, gapAfter } = mergeEvents(state, incoming)
      if (next !== state) {
        state = next
        keep(id, next)
        setView((v) => ({ ...v, state: next }))
      }
      if (gapAfter === null || filling) return
      filling = true
      api
        .chatHistory(id, gapAfter)
        .then((events) => {
          if (!gone) apply(events)
        }, fail)
        .finally(() => (filling = false))
    }
    const flush = () => {
      frame = 0
      const events = held
      held = []
      apply(events)
    }

    const off = on(chatEvent(id), ({ events }) => {
      held.push(...events)
      if (loaded && !frame) frame = requestAnimationFrame(flush)
    })
    // An ended thread cannot be watched, and its history is all there is.
    inTurn(id, () => api.chatWatch(id).catch(() => undefined))
      .then(() => api.chatHistory(id, state.last))
      .then((events) => {
        if (gone) return
        loaded = true
        held = [...events, ...held]
        flush()
        setView((v) => ({ ...v, loading: false }))
      }, fail)

    return () => {
      gone = true
      off()
      cancelAnimationFrame(frame)
      inTurn(id, () => api.chatUnwatch(id)).catch(() => undefined)
    }
  }, [id])

  return { timeline, loading: shown.loading, error: shown.error }
}
