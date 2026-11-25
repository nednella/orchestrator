import { useEffect, useRef, useSyncExternalStore } from 'react'
import { setOpened } from './chatOpen'
import { agentList } from './chatTimeline'
import type { AgentItem, TimelineItem } from './chatTimeline'
import type { Session } from './types'

// The subagents of each thread whose chat is shown, for its Agents list, and the asks to open one of them in the chat.
// The chat keeps them here, so the list renders again only when one of them changes.
const shown = new Map<string, AgentItem[]>()
const listeners = new Set<() => void>()
const openers = new Map<string, Set<(agent: string) => void>>()
const none: AgentItem[] = []

const sameAgent = (a: AgentItem, b: AgentItem) => a.id === b.id && a.description === b.description && a.state === b.state && a.startedAt === b.startedAt && a.endedAt === b.endedAt && a.tools === b.tools

// showAgents keeps the thread's subagents, in the order its list shows them; an empty list forgets them.
export function showAgents(thread: string, agents: AgentItem[]) {
  const before = shown.get(thread) ?? none
  if (before.length === agents.length && before.every((a, i) => sameAgent(a, agents[i]))) return
  if (agents.length === 0) shown.delete(thread)
  else shown.set(thread, agents)
  listeners.forEach((listener) => listener())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

// useThreadAgents is the subagents of the thread's chat, none while its chat is not shown.
export function useThreadAgents(thread: string): AgentItem[] {
  return useSyncExternalStore(subscribe, () => shown.get(thread) ?? none)
}

// agentsTabShown is whether the thread's details show its Agents tab: when its chat has a subagent or it has a child
// thread, among sessions.
export function agentsTabShown(thread: string, sessions: Session[]): boolean {
  return shown.has(thread) || sessions.some((s) => s.parent === thread)
}

// openAgent opens the subagent's row in the thread's chat and brings it into view.
export function openAgent(thread: string, agent: string) {
  setOpened(thread, agent, true)
  openers.get(thread)?.forEach((open) => open(agent))
}

// onAgentOpen calls open with each subagent of the thread that is asked to open, until the returned function is called.
export function onAgentOpen(thread: string, open: (agent: string) => void): () => void {
  const set = openers.get(thread) ?? new Set()
  openers.set(thread, set)
  set.add(open)
  return () => {
    set.delete(open)
    if (set.size === 0) openers.delete(thread)
  }
}

// useShowAgents keeps the subagents of the thread's chat for its list while the chat is shown, and brings the row of
// one the list opens into view with scrollTo, by its index.
export function useShowAgents(thread: string, items: TimelineItem[], scrollTo: (index: number) => void) {
  const latest = useRef(items)
  useEffect(() => {
    latest.current = items
    showAgents(thread, agentList(items))
  }, [thread, items])
  useEffect(() => () => showAgents(thread, []), [thread])
  useEffect(
    () =>
      onAgentOpen(thread, (agent) => {
        const index = latest.current.findIndex((item) => item.kind === 'agent' && item.id === agent)
        if (index >= 0) scrollTo(index)
      }),
    [thread, scrollTo],
  )
}
