import { useAgentos } from '../AgentosContext'
import { stateLabel } from '../stateMeta'
import type { Session } from '../types'
import { StateDot } from './StateDot'

type IssueThreadProps = { session: Session }

export function IssueThread({ session }: IssueThreadProps) {
  const { select } = useAgentos('select')
  return (
    <span className="flex min-w-0 items-center gap-1.5 text-dim">
      <StateDot state={session.state} />
      <span className="truncate">{stateLabel(session.state, session.chat).toLowerCase()}</span>
      <span aria-hidden="true">·</span>
      <button className="flex-none font-medium text-ink hover:underline" title={`Open thread ${session.n}`} onClick={() => select(session.id)}>
        Open
      </button>
    </span>
  )
}
