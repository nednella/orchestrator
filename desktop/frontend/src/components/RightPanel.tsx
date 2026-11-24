import { useAgentos, useAgentosSelector } from '../AgentosContext'
import type { SessionView } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { BrowserTab } from './BrowserTab'
import { EvidenceTab } from './EvidenceTab'
import { Icon } from './Icon'
import { PRTab } from './PRTab'
import { ThreadFacts } from './ThreadFacts'

export function RightPanel() {
  const { selectedId, viewOf, setSessionView, browserStates } = useAgentos('selectedId', 'viewOf', 'setSessionView', 'browserStates')
  const session = useAgentosSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const { widths, setRightOpen, mode } = useLayout()
  const narrow = mode === 'narrow'
  if (!session) return null
  const view = viewOf(selectedId)
  const browserOpen = browserStates[session.id]?.open ?? session.browser
  const tabs: { id: SessionView; label: string; count?: number; live?: boolean }[] = [
    { id: 'pr', label: 'PR' },
    { id: 'evidence', label: 'Evidence', count: session.evidence },
    { id: 'browser', label: 'Browser', live: browserOpen },
  ]

  return (
    <aside data-panel="right" tabIndex={-1} className={`panel flex h-full min-h-0 flex-col overflow-hidden ${narrow ? 'min-w-0 flex-1' : 'flex-none'}`} style={narrow ? undefined : { width: `${widths.right}rem` }}>
      <ThreadFacts session={session} />
      <div className="flex flex-none items-center border-b border-line pr-2 pl-2">
        <div role="tablist" aria-label="Thread details" className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
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
        <button className="btn btn-ghost h-8 w-8 flex-none justify-center px-0" title="Close the right panel" aria-label="Close the right panel" onClick={() => setRightOpen(false)}>
          <Icon name="close" />
        </button>
      </div>
      <div className="relative min-h-0 flex-1">
        {view === 'pr' && <PRTab key={session.id} session={session} />}
        {view === 'evidence' && <EvidenceTab key={session.id} session={session} />}
        {view === 'browser' && <BrowserTab key={session.id} session={session} />}
      </div>
    </aside>
  )
}
