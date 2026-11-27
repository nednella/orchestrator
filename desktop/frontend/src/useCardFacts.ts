import { useOrchestrator } from './OrchestratorContext'
import type { Orchestrator } from './OrchestratorContext'
import { useThreadAgents } from './chatAgents'
import type { AgentItem } from './chatTimeline'
import { cardLineage, cardSections } from './contextCard'
import type { CardFacts, CardSection } from './contextCard'
import type { Changes, Server, Thread } from './types'
import { useThreadChanges } from './useThreadChanges'
import { useThreadServers } from './useThreadServers'

type Known = Pick<Orchestrator, 'machine' | 'threads' | 'sideChats' | 'issues' | 'project' | 'settings'>

function factsOf(thread: Thread, a: Known, servers: Server[], agents: AgentItem[], changes: Changes | null): CardFacts {
  return {
    machine: a.machine,
    folder: thread.worktree || a.project?.dir || '',
    editor: a.settings.editorApp,
    issueTitle: a.issues.find((i) => i.number === thread.issue)?.title ?? '',
    changes,
    servers,
    agents,
    ...cardLineage(thread, a.threads),
    sideChats: a.sideChats.filter((s) => s.forkOf === thread.id),
  }
}

// useCardFacts gathers what the context card of thread id shows: the thread, its sections, and a read of its servers
// again. The servers and the changes are read when the card opens and at each turn end, when a thread starts or stops
// one or changes files.
export function useCardFacts(id: string): { thread: Thread | undefined; sections: CardSection[]; refreshServers(): Promise<void> } {
  const a = useOrchestrator('machine', 'threads', 'sideChats', 'issues', 'project', 'settings')
  const thread = a.threads.find((s) => s.id === id)
  const agents = useThreadAgents(id)
  const { servers, refresh } = useThreadServers(id, thread?.state, false)
  const changes = useThreadChanges(id, thread?.state)
  const sections = thread ? cardSections(thread, factsOf(thread, a, servers, agents, changes)) : []
  return { thread, sections, refreshServers: refresh }
}

// selectedCardHasRows says whether the open thread's card shows anything, for useOrchestratorSelector. It leaves out the
// servers and subagents, which it would have to read: a thread that runs them has a folder, which is a row already.
export function selectedCardHasRows(a: Orchestrator): boolean {
  const thread = a.threads.find((s) => s.id === a.selectedId)
  return thread !== undefined && cardSections(thread, factsOf(thread, a, [], [], null)).length > 0
}
