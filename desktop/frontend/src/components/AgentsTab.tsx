import { Bot } from 'lucide-react'
import { openAgent, useThreadAgents } from '../chatAgents'
import type { AgentState } from '../chatTimeline'
import { useLayout } from '../LayoutContext'
import { Ago } from './Ago'

type AgentsTabProps = { sessionId: string }

const stateLabel: Record<AgentState, string> = { running: 'Running', done: 'Done', failed: 'Failed', stopped: 'Stopped' }

// AgentsTab lists what works for the thread beside it: the subagents Claude started in its chat. A click opens one
// in the chat.
export function AgentsTab({ sessionId }: AgentsTabProps) {
  const { closeOverlay } = useLayout()
  const agents = useThreadAgents(sessionId)
  if (agents.length === 0) {
    return (
      <div className="grid h-full place-items-center p-6 text-center">
        <p className="max-w-xs text-soft">No subagents yet. Each one Claude starts in this thread appears here.</p>
      </div>
    )
  }
  return (
    <div className="flex h-full flex-col overflow-y-auto py-1">
      <div className="flex h-8 flex-none items-center gap-2 px-3.5">
        <span className="label">Subagents</span>
        <span className="mono text-small text-dim">{agents.length}</span>
      </div>
      {agents.map((a) => (
        <button
          key={a.id}
          className="flex h-8 w-full min-w-0 flex-none items-center gap-2 px-3.5 text-left text-body hover:bg-hover"
          onClick={() => {
            openAgent(sessionId, a.id)
            closeOverlay('thread-focused')
          }}
        >
          <span className={`flex flex-none ${a.state === 'failed' ? 'text-danger' : 'text-dim'}`}>
            <Bot size={14} strokeWidth={2} aria-hidden />
          </span>
          <span className="min-w-0 truncate">{a.description}</span>
          <span className="ml-auto flex flex-none items-center gap-2 text-small text-dim tabular-nums">
            <span className={a.state === 'failed' ? 'text-danger' : undefined}>{stateLabel[a.state]}</span>
            <Ago at={a.startedAt} />
          </span>
        </button>
      ))}
    </div>
  )
}
