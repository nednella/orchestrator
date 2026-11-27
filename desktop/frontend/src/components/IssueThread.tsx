import { ArrowRight } from 'lucide-react'
import { useOrchestrator } from '../OrchestratorContext'
import { STATE_LABEL } from '../stateMeta'
import type { Thread } from '../types'
import { StateDot } from './StateDot'

type IssueThreadProps = { thread: Thread }

export function IssueThread({ thread }: IssueThreadProps) {
  const { select } = useOrchestrator('select')
  return (
    <span className="flex min-w-0 items-center gap-1.5 text-dim">
      <StateDot state={thread.state} />
      <span className="truncate">{STATE_LABEL[thread.state]}</span>
      <button className="flex flex-none items-center gap-0.5 pl-1.5 font-medium text-ink hover:underline" title={`Open thread ${thread.n}`} onClick={() => select(thread.id)}>
        Open
        <ArrowRight size={12} aria-hidden="true" />
      </button>
    </span>
  )
}
