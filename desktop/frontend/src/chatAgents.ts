import { useEffect, useSyncExternalStore } from 'react'
import { agentList } from './chatTimeline'
import type { AgentItem, TimelineItem } from './chatTimeline'
import { closedIn, heldBy, keptFor, viewedIn } from './subagentView'
import type { AgentView } from './subagentView'

// The subagents of each thread whose chat is shown, for its Agents list, and the subagent whose view is open. The chat
// keeps them here, and the list renders again only when what it shows of one of them changes: a subagent's tool calls
// change nothing in it.
const shown = new Map<string, AgentItem[]>()
const listeners = new Set<() => void>()
const none: AgentItem[] = []
let viewed: AgentView | null = null

const sameAgent = (a: AgentItem, b: AgentItem) => a.id === b.id && a.description === b.description && a.state === b.state && a.startedAt === b.startedAt

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

// useAgentsRunning is how many subagents of the thread's chat work now, null while it has none, for the panel's list of
// views, which renders again only when that changes.
export function useAgentsRunning(thread: string): number | null {
  return useSyncExternalStore(subscribe, () => {
    const agents = shown.get(thread)
    return agents ? agents.filter((a) => a.state === 'running').length : null
  })
}

// view sets the subagent view and tells the listeners when it changed.
function view(next: AgentView | null) {
  if (next === viewed) return
  viewed = next
  listeners.forEach((listener) => listener())
}

// openAgent opens the subagent's own view in place of the thread's chat.
export function openAgent(thread: string, agent: string) {
  view({ thread, agent })
}

// closeAgent takes the thread from its subagent's view back to its chat.
export function closeAgent(thread: string) {
  view(closedIn(viewed, thread))
}

// keepAgentViewOf closes a subagent view of any thread but the one on screen.
export function keepAgentViewOf(selected: string) {
  view(keptFor(viewed, selected))
}

// closeGoneAgent closes the thread's subagent view once its loaded chat no longer holds the subagent.
export function closeGoneAgent(thread: string, loaded: boolean, holds: boolean) {
  view(heldBy(viewed, thread, loaded, holds))
}

// useAgentView is the subagent view that is open, of any thread.
export function useAgentView(): AgentView | null {
  return useSyncExternalStore(subscribe, () => viewed)
}

// useViewedAgent is the subagent of the thread whose view is open, null while the thread shows its chat.
export function useViewedAgent(thread: string): string | null {
  return useSyncExternalStore(subscribe, () => viewedIn(viewed, thread))
}

// useShowAgents keeps the subagents of the thread's chat for its list while the chat, or one of its subagents' views,
// is shown.
export function useShowAgents(thread: string, items: TimelineItem[]) {
  useEffect(() => showAgents(thread, agentList(items)), [thread, items])
  useEffect(() => () => showAgents(thread, []), [thread])
}
