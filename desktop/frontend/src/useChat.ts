import { useEffect, useMemo, useRef, useState } from 'react'
import { useLatestOrchestrator } from './OrchestratorContext'
import { api, chatEvent, errorMessage, on } from './api'
import { EMPTY_CHAT, chatFeed } from './chatEvents'
import { watchAgain } from './chatLive'
import type { ChatState } from './chatEvents'
import { buildTimeline } from './chatTimeline'
import { ownStart } from './sideChat'
import type { Timeline } from './chatTimeline'
import type { ChatEvent, State } from './types'

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
  return { id, state: state ? { ...state, live: {}, thinking: {} } : EMPTY_CHAT, loading: !state, error: '' }
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

// later runs on the next frame, or after 250 ms if no frame comes, as in a hidden window.
function later(run: () => void): () => void {
  const timer = setTimeout(() => {
    cancelAnimationFrame(frame)
    run()
  }, 250)
  const frame = requestAnimationFrame(() => {
    clearTimeout(timer)
    run()
  })
  return () => {
    clearTimeout(timer)
    cancelAnimationFrame(frame)
  }
}

// useChat is the thread's chat: its history, then its live events, as a timeline, and the logged events it is built
// from. state is the thread's row state: a settled thread that wakes is watched again. A side chat's timeline leaves
// out the chat it was forked with, which is its thread's.
export function useChat(id: string, state: State, side = false): { timeline: Timeline; events: ChatEvent[]; loading: boolean; error: string } {
  const [view, setView] = useState(() => startOf(id))
  const latest = useLatestOrchestrator()
  const seen = useRef<{ id: string; state: State }>({ id, state })
  const [wakes, setWakes] = useState(0)
  useEffect(() => {
    const before = seen.current.id === id ? seen.current.state : undefined
    seen.current = { id, state }
    if (watchAgain(before, state)) setWakes((n) => n + 1)
  }, [id, state])
  const shown = view.id === id ? view : startOf(id)
  const timeline = useMemo(() => {
    const { events } = shown.state
    return buildTimeline(side ? events.slice(ownStart(events)) : events, shown.state.live, id, shown.state.thinking)
  }, [shown.state, id, side])

  useEffect(() => {
    const start = startOf(id)
    setView(start)
    let gone = false
    const feed = chatFeed(start.state, {
      history: (after) => api.chatHistory(id, after),
      later,
      onState(state) {
        keep(id, state)
        setView((v) => ({ ...v, state }))
      },
      onError: (err) => setView((v) => ({ ...v, loading: false, error: errorMessage(err) })),
    })
    const off = on(chatEvent(id), ({ events }) => feed.push(events))
    inTurn(id, () => api.chatWatch(id))
      .catch((err) => {
        // A settled thread cannot be watched, and its history is all there is.
        const { sessions, sideChats } = latest()
        if (gone || (sessions.find((s) => s.id === id) ?? sideChats.find((s) => s.id === id))?.state === 'settled') return
        setView((v) => ({ ...v, error: errorMessage(err) }))
      })
      .then(() => feed.load())
      .then(() => {
        if (!gone) setView((v) => ({ ...v, loading: false }))
      })

    return () => {
      gone = true
      feed.stop()
      off()
      inTurn(id, () => api.chatUnwatch(id)).catch(() => undefined)
    }
  }, [id, latest, wakes])

  return { timeline, events: shown.state.events, loading: shown.loading, error: shown.error }
}
