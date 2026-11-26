import { useAgentos } from '../AgentosContext'
import { useThreadAgents } from '../chatAgents'
import { useLayout } from '../LayoutContext'
import type { PanelViewProps } from '../panelViews'
import { AGENT_STATE_LABEL } from '../stateMeta'
import { agentRows } from '../threads'
import { AgentThreadRow } from './AgentThreadRow'
import { Ago } from './Ago'
import { ClaudeMark } from './ClaudeMark'
import { Notice } from './Notice'

// AgentsView lists what works for the thread: the subagents Claude started in its chat, each of which opens in a view
// of its own, then the threads it delegated to, each under the thread that delegated it.
export function AgentsView({ session }: PanelViewProps) {
  const { sessions, openAgent } = useAgentos('sessions', 'openAgent')
  const { focusPanel } = useLayout()
  const agents = useThreadAgents(session?.id ?? '')
  if (!session) return <Notice title="Available in a thread" hint="Open a thread to see its subagents and the threads it delegated to." centered />
  const children = agentRows(session.id, sessions)
  if (agents.length === 0 && children.length === 0) {
    return <Notice title="No agents yet" hint="A subagent Claude starts in this thread, or a thread it delegates to, shows here." centered />
  }

  return (
    <div className="flex h-full flex-col overflow-y-auto p-2">
      {agents.length > 0 && (
        <h3 className="flex h-8 flex-none items-center gap-2 px-2.5">
          <span className="label">Subagents</span>
          <span className="mono text-small text-dim">{agents.length}</span>
        </h3>
      )}
      {agents.map((agent) => (
        <button
          key={agent.id}
          className="flex h-8 w-full min-w-0 flex-none items-center gap-2.5 rounded-lg px-2.5 text-left text-body hover:bg-(--bg-row-hover)"
          onClick={() => {
            openAgent(session.id, agent.id)
            focusPanel('centre')
          }}
        >
          <span className={`flex flex-none ${agent.state === 'failed' ? 'text-danger' : 'text-soft'}`}>
            <ClaudeMark size={16} />
          </span>
          <span className="min-w-0 truncate">{agent.description}</span>
          <span className="ml-auto flex flex-none items-center gap-2 text-small text-dim tabular-nums">
            <span className={agent.state === 'failed' ? 'text-danger' : undefined}>{AGENT_STATE_LABEL[agent.state]}</span>
            <Ago at={agent.startedAt} />
          </span>
        </button>
      ))}
      {children.length > 0 && (
        <h3 className="flex h-8 flex-none items-center gap-2 px-2.5">
          <span className="label">Delegated</span>
          <span className="mono text-small text-dim">{children.length}</span>
        </h3>
      )}
      {children.map(({ session: child, depth }) => (
        <AgentThreadRow key={child.id} child={child} depth={depth} />
      ))}
    </div>
  )
}
