import { useAgentos } from '../AgentosContext'
import type { SessionView } from '../AgentosContext'
import { agentsTabShown, useThreadAgents } from '../chatAgents'
import type { PanelContext } from '../panelViews'
import { AgentsTab } from './AgentsTab'
import { BrowserTab } from './BrowserTab'
import { EvidenceTab } from './EvidenceTab'
import { PRTab } from './PRTab'
import { ThreadFacts } from './ThreadFacts'

type ThreadDetailsViewProps = { ctx: PanelContext }

export function ThreadDetailsView({ ctx: { session } }: ThreadDetailsViewProps) {
  const { viewOf, setSessionView, browserStates, sessions } = useAgentos('viewOf', 'setSessionView', 'browserStates', 'sessions')
  const agents = useThreadAgents(session?.id ?? '')
  if (!session) return null
  const withAgents = agentsTabShown(session.id, sessions)
  const chosen = viewOf(session.id)
  const view = chosen === 'agents' && !withAgents ? 'pr' : chosen
  const browserOpen = browserStates[session.id]?.open ?? session.browser
  const running = agents.filter((a) => a.state === 'running').length + sessions.filter((s) => s.parent === session.id && s.state === 'working').length
  const tabs: { id: SessionView; label: string; count?: number; live?: boolean }[] = [
    { id: 'pr', label: 'PR' },
    { id: 'evidence', label: 'Evidence', count: session.evidence },
    { id: 'browser', label: 'Browser', live: browserOpen },
    ...(withAgents ? [{ id: 'agents' as const, label: 'Agents', count: running }] : []),
  ]

  return (
    <div className="flex h-full min-h-0 flex-col">
      <ThreadFacts session={session} />
      <div role="tablist" aria-label="Thread details" className="flex flex-none items-center gap-1 overflow-x-auto border-b border-line px-2">
        {tabs.map((tab) => {
          const active = view === tab.id
          return (
            <button
              key={tab.id}
              role="tab"
              aria-selected={active}
              className="flex h-10 flex-none items-center gap-1.5 border-b-2 px-2 text-body font-medium"
              style={{ borderColor: active ? 'var(--accent)' : 'transparent', color: active ? 'var(--text)' : 'var(--text-soft)' }}
              onClick={() => setSessionView(session.id, tab.id)}
            >
              {tab.label}
              {tab.live && <span className="dot" data-state="working" title="Browser open" />}
              {tab.count !== undefined && tab.count > 0 && <span className="mono text-small text-dim">({tab.count})</span>}
            </button>
          )
        })}
      </div>
      <div className="relative min-h-0 flex-1">
        {view === 'pr' && <PRTab key={session.id} session={session} />}
        {view === 'evidence' && <EvidenceTab key={session.id} session={session} />}
        {view === 'browser' && <BrowserTab key={session.id} session={session} />}
        {view === 'agents' && <AgentsTab key={session.id} sessionId={session.id} agents={agents} />}
      </div>
    </div>
  )
}
