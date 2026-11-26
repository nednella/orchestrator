import { useAgentos } from './AgentosContext'
import type { Agentos } from './AgentosContext'
import { useThreadAgents } from './chatAgents'
import type { AgentItem } from './chatTimeline'
import { cardLineage, cardSections } from './contextCard'
import type { CardFacts, CardSection } from './contextCard'
import type { Server, Session } from './types'
import { useThreadServers } from './useThreadServers'

type Known = Pick<Agentos, 'machine' | 'sessions' | 'issues' | 'project' | 'settings'>

function factsOf(session: Session, a: Known, servers: Server[], agents: AgentItem[]): CardFacts {
  return {
    machine: a.machine,
    folder: session.worktree || a.project?.dir || '',
    editor: a.settings.editorApp,
    issueTitle: a.issues.find((i) => i.number === session.issue)?.title ?? '',
    servers,
    agents,
    ...cardLineage(session, a.sessions),
  }
}

// useCardFacts gathers what the context card of thread id shows: the thread, its sections, and a read of its servers
// again. The servers are read when the card opens and at each turn end, when a thread starts
// or stops one.
export function useCardFacts(id: string): { session: Session | undefined; sections: CardSection[]; refreshServers(): Promise<void> } {
  const a = useAgentos('machine', 'sessions', 'issues', 'project', 'settings')
  const session = a.sessions.find((s) => s.id === id)
  const agents = useThreadAgents(id)
  const { servers, refresh } = useThreadServers(id, session?.state, false)
  const sections = session ? cardSections(session, factsOf(session, a, servers, agents)) : []
  return { session, sections, refreshServers: refresh }
}

// selectedCardHasRows says whether the open thread's card shows anything, for useAgentosSelector. It leaves out the
// servers and subagents, which it would have to read: a thread that runs them has a folder, which is a row already.
export function selectedCardHasRows(a: Agentos): boolean {
  const session = a.sessions.find((s) => s.id === a.selectedId)
  return session !== undefined && cardSections(session, factsOf(session, a, [], [])).length > 0
}
