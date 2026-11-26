import { Bot, ChevronDown, ChevronRight } from 'lucide-react'
import { memo, useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { isOpened, setOpened } from '../chatOpen'
import { sameSteps } from '../chatTimeline'
import type { AgentState, WorkStep } from '../chatTimeline'
import { formatWorked } from '../chatTools'
import { ChatMarkdown } from './ChatMarkdown'
import { ChatShimmer } from './ChatShimmer'
import { ChatSteps } from './ChatSteps'
import { WorkingTime } from './WorkingTime'

type ChatAgentRowProps = { thread: string; id: string; description: string; type: string; state: AgentState; startedAt: number; endedAt: number | null; tools: number; steps: WorkStep[]; result: string | null; folder: string }

const stateLabel: Record<AgentState, string> = { running: 'Running', done: 'Done', failed: 'Failed', stopped: 'Stopped' }

// The timeline is built anew on every event, so a subagent's steps are new objects holding the same values, and an
// event of another subagent leaves this row as it was.
const unchanged = (a: ChatAgentRowProps, b: ChatAgentRowProps) =>
  a.thread === b.thread && a.id === b.id && a.description === b.description && a.type === b.type && a.state === b.state && a.startedAt === b.startedAt && a.endedAt === b.endedAt && a.tools === b.tools && a.result === b.result && a.folder === b.folder && sameSteps(a.steps, b.steps)

// ChatAgentRow is a subagent Claude started, on one line: what it does, its type, how long it has worked and how many
// tools it ran. The line opens the subagent's own view; its chevron shows what the subagent did, under the line, and
// its report.
export const ChatAgentRow = memo(function ChatAgentRow({ thread, id, description, type, state, startedAt, endedAt, tools, steps, result, folder }: ChatAgentRowProps) {
  const { openAgent } = useOrchestrator('openAgent')
  const [open, setOpen] = useState(() => isOpened(thread, id))
  const running = state === 'running'
  const Chevron = open ? ChevronDown : ChevronRight

  return (
    <div>
      <div className="flex min-w-0 items-center gap-2 text-small text-soft">
        <button className="flex min-w-0 flex-1 items-center gap-2 py-1 text-left hover:text-ink" title={`Open ${description}`} onClick={() => openAgent(thread, id)}>
          <span className={`flex flex-none ${state === 'failed' ? 'text-danger' : 'text-dim'}`}>
            <Bot size={12} strokeWidth={2} aria-hidden />
          </span>
          {running ? <ChatShimmer text={description} /> : <span className="min-w-0 truncate">{description}</span>}
          {type && <span className="flex-none text-dim">{type}</span>}
          <span className="ml-auto flex flex-none items-center gap-2 text-dim tabular-nums">
            {!running && <span className={state === 'failed' ? 'text-danger' : undefined}>{stateLabel[state]}</span>}
            <Worked startedAt={startedAt} endedAt={endedAt} />
            {tools > 0 && <span>{tools === 1 ? '1 tool' : `${tools} tools`}</span>}
          </span>
        </button>
        <button
          className="-mr-1 flex flex-none rounded-sm p-1 text-dim hover:text-ink"
          aria-expanded={open}
          aria-label={open ? 'Hide its steps' : 'Show its steps'}
          title={open ? 'Hide its steps' : 'Show its steps'}
          onClick={() => {
            setOpened(thread, id, !open)
            setOpen(!open)
          }}
        >
          <Chevron size={12} aria-hidden />
        </button>
      </div>
      {open && (
        <div className="mb-2 ml-1.5 border-l border-line pl-3.5">
          <ChatSteps thread={thread} steps={steps} running={running} folder={folder} />
          {result && (
            <div className={`py-1 text-small ${state === 'failed' ? 'text-danger' : 'text-ink'}`}>
              <ChatMarkdown text={result} />
            </div>
          )}
        </div>
      )}
    </div>
  )
}, unchanged)

type WorkedProps = { startedAt: number; endedAt: number | null }

// Worked is how long the subagent worked, or has worked so far.
function Worked({ startedAt, endedAt }: WorkedProps) {
  if (endedAt === null) return <WorkingTime since={startedAt} />
  return <span>{formatWorked(endedAt - startedAt, true)}</span>
}
