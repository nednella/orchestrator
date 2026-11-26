import { Bot, CircleDot, Files, GitPullRequestArrow, Globe, Image, Server, StickyNote } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { ComponentType } from 'react'
import { useAgentos } from './AgentosContext'
import { useAgentsRunning, useThreadAgents } from './chatAgents'
import { AgentsView } from './components/AgentsView'
import { BrowserView } from './components/BrowserView'
import { EvidenceView } from './components/EvidenceView'
import { FilesView } from './components/FilesView'
import { IssuesView } from './components/IssuesView'
import { NotesView } from './components/NotesView'
import { PullRequestsView } from './components/PullRequestsView'
import { ServersView } from './components/ServersView'
import { PANEL_ORDER } from './panelTabs'
import type { PanelViewId } from './panelTabs'
import { descendantsOf } from './threads'
import type { Session } from './types'
import { useThreadServers } from './useThreadServers'

// session is the open thread, none on Home.
export type PanelViewProps = { session: Session | undefined }

// thread says the view shows one thread's things: Home lists it, but cannot open it.
export type PanelView = { id: PanelViewId; label: string; icon: LucideIcon; thread: boolean; Component: ComponentType<PanelViewProps> }

const VIEWS: Partial<Record<PanelViewId, PanelView>> = {
  agents: { id: 'agents', label: 'Agents', icon: Bot, thread: true, Component: AgentsView },
  servers: { id: 'servers', label: 'Servers', icon: Server, thread: true, Component: ServersView },
  pulls: { id: 'pulls', label: 'Pull requests', icon: GitPullRequestArrow, thread: false, Component: PullRequestsView },
  issues: { id: 'issues', label: 'Issues', icon: CircleDot, thread: false, Component: IssuesView },
  notes: { id: 'notes', label: 'Notes', icon: StickyNote, thread: false, Component: NotesView },
  files: { id: 'files', label: 'Files', icon: Files, thread: false, Component: FilesView },
  evidence: { id: 'evidence', label: 'Evidence', icon: Image, thread: true, Component: EvidenceView },
  browser: { id: 'browser', label: 'Browser', icon: Globe, thread: true, Component: BrowserView },
}

export const panelView = (id: PanelViewId): PanelView | undefined => VIEWS[id]

// The panel's list of views, in its order.
export const LISTED_VIEWS: PanelView[] = PANEL_ORDER.flatMap((id) => VIEWS[id] ?? [])

// agentsWait says whether the Agents view of s holds a thread that waits on the owner.
export function agentsWait(s: Session | undefined, sessions: Session[]): boolean {
  return s !== undefined && descendantsOf(s.id, sessions).some((d) => d.state === 'waiting')
}

// usePanelMeta is the short fact the list shows beside each view, '' for none.
export function usePanelMeta(s: Session | undefined): Partial<Record<PanelViewId, string>> {
  const { sessions, issues, notes, browserStates } = useAgentos('sessions', 'issues', 'notes', 'browserStates')
  const id = s?.id ?? ''
  const subagents = useThreadAgents(id)
  const subagentsRunning = useAgentsRunning(id) ?? 0
  const { servers } = useThreadServers(id, s?.state, false)
  const family = s ? descendantsOf(s.id, sessions) : []
  const running = subagentsRunning + family.filter((d) => d.state === 'working').length
  const agents = subagents.length + family.length
  const shown = notes.filter((n) => !n.archived).length
  return {
    agents: running > 0 ? `${running} running` : agents > 0 ? String(agents) : '',
    servers: servers.length > 0 ? `${servers.length} running` : '',
    pulls: s?.pr ? `#${s.pr.number} ${s.pr.state}` : '',
    issues: issues.length > 0 ? `${issues.length} open` : '',
    notes: shown > 0 ? String(shown) : '',
    evidence: s && s.evidence > 0 ? String(s.evidence) : '',
    browser: s && (browserStates[s.id]?.open ?? s.browser) ? 'Open' : '',
  }
}
