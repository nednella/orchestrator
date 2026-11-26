import { ArrowUpLeft } from 'lucide-react'
import type { AgentItem } from '../chatTimeline'
import { modelName } from '../model'
import { subagentStatus } from '../subagentView'
import { useClock } from '../time'
import { ClaudeMark } from './ClaudeMark'

// parentModel is the model of the thread that started the subagent, which a subagent runs on when its call names none.
type SubagentBarProps = { agent: AgentItem; parentModel: string; onOpenParent(): void }

// SubagentBar stands where the composer stands in a subagent's view: nothing can be sent to a subagent, so it says
// what runs it and how far it is, with the way back to its thread.
export function SubagentBar({ agent, parentModel, onOpenParent }: SubagentBarProps) {
  const status = useClock((now) => subagentStatus(agent, now))
  return (
    <div className="@container">
      <div className="composer-box flex min-h-12 flex-wrap items-center gap-x-3 py-2 pr-2 pl-5 text-body">
        <ClaudeMark size={16} />
        <span className="min-w-0 truncate font-medium">{modelName(agent.model || parentModel) || 'Subagent'}</span>
        <span className={`tabular-nums @max-[560px]:order-last @max-[560px]:basis-full @max-[560px]:text-small ${agent.state === 'failed' ? 'text-danger' : 'text-soft'}`}>{status}</span>
        <span className="text-soft @max-[640px]:hidden">Runs on its own</span>
        <button className="btn btn-ghost ml-auto" title="Open parent" aria-label="Open parent" onClick={onOpenParent}>
          <ArrowUpLeft size={14} aria-hidden="true" />
          <span className="@max-[320px]:hidden">Open parent</span>
        </button>
      </div>
    </div>
  )
}
