import { useAgentos } from './AgentosContext'
import { useThreadAgents } from './chatAgents'
import { cardSections } from './contextCard'
import type { CardSection } from './contextCard'
import type { Session } from './types'
import { useThreadServers } from './useThreadServers'

// The app opens a thread's folder with the config's editor_app, which it does not tell the front end; the palette's
// action names VS Code too.
const EDITOR = 'VS Code'

// useCardFacts gathers what the thread's context card shows: its sections, the address of its issue, and a read of its
// servers again. The servers are read when the card opens and at each turn end, when a thread starts or stops one.
export function useCardFacts(session: Session): { sections: CardSection[]; issueURL: string; refreshServers(): Promise<void> } {
  const { machine, sessions, issues, project } = useAgentos('machine', 'sessions', 'issues', 'project')
  const agents = useThreadAgents(session.id)
  const { servers, refresh } = useThreadServers(session.id, session.state, false)
  const issue = issues.find((i) => i.number === session.issue)
  const sections = cardSections(session, {
    machine,
    folder: session.worktree || project?.dir || '',
    editor: EDITOR,
    issueTitle: issue?.title ?? '',
    servers,
    agents,
    children: sessions.filter((s) => s.parent === session.id),
    parent: sessions.find((s) => s.id === (session.parent || session.forkOf)) ?? null,
    forks: sessions.filter((s) => s.forkOf === session.id),
  })
  const issueURL = issue?.url ?? (project?.repo ? `https://github.com/${project.repo}/issues/${session.issue}` : '')
  return { sections, issueURL, refreshServers: refresh }
}
