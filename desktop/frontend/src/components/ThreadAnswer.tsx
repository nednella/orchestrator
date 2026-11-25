import { useAgentos } from '../AgentosContext'
import type { Pending, Session } from '../types'
import { useAnswering } from '../useAnswering'

// The buttons sit above the card's own button, so a click on one answers without opening the thread.
type ThreadAnswerProps = { session: Session; pending: Pending }

export function ThreadAnswer({ session, pending }: ThreadAnswerProps) {
  const a = useAgentos('answer', 'select', 'focus')
  const { busy, send } = useAnswering()

  if (pending.kind !== 'tool') {
    return (
      <span className="relative z-[1] flex">
        <button
          className="btn btn-primary h-[1.375rem] px-2 text-label"
          onClick={() => {
            a.select(session.id)
            a.focus('terminal')
          }}
        >
          Answer
        </button>
      </span>
    )
  }
  return (
    <span className="relative z-[1] flex gap-1.5">
      <button className="btn btn-primary h-[1.375rem] px-2 text-label" disabled={busy} onClick={() => send(() => a.answer(session.id, pending.request, true))}>
        Allow
      </button>
      <button className="btn h-[1.375rem] px-2 text-label" disabled={busy} onClick={() => send(() => a.answer(session.id, pending.request, false))}>
        Deny
      </button>
    </span>
  )
}
