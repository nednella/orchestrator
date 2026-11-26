import { useState } from 'react'
import { useAgentos } from '../AgentosContext'
import { cleanupForced, cleanupRequest, passingReason } from '../cleanupConfirm'
import { keptLine } from '../cleanupText'
import { confirmed } from '../confirm'
import { readStored, writeStored } from '../storage'
import type { Session } from '../types'

type CleanupControlsProps = { session: Session }

const KEPT_KEY = 'orchestrator.keptCleanups'

export function CleanupControls({ session }: CleanupControlsProps) {
  const { cleanupSession, report } = useAgentos('cleanupSession', 'report')
  const [kept, setKept] = useState(() => readStored<string[]>(KEPT_KEY, []).includes(session.id))
  const { cleanup, cleanupReason } = session

  if (cleanup === 'pending') return <span className="text-small text-dim">cleaning up…</span>

  const remove = () => report(() => confirmed(cleanupRequest(session), () => cleanupSession(session.id, cleanupForced(cleanup))))

  if (cleanup === 'ask' && kept) return null

  if (cleanup === 'blocked') {
    return (
      <div className="flex w-full flex-wrap items-center gap-x-2 gap-y-1">
        <span className="min-w-0 text-small text-danger">{keptLine(cleanupReason)}</span>
        {passingReason(cleanupReason) && (
          <button className="btn h-6" title="Run every check again" onClick={() => report(() => cleanupSession(session.id, false))}>
            Clean up
          </button>
        )}
        <button className="btn h-6" onClick={remove}>
          Clean up anyway
        </button>
      </div>
    )
  }

  return (
    <div className="flex w-full flex-wrap items-center gap-x-2 gap-y-1">
      <span className="text-small text-waiting">
        {session.pr?.state === 'merged' ? 'PR was already merged' : 'PR closed without merging'}
      </span>
      <button className="btn h-6" onClick={remove}>
        Clean up
      </button>
      <button
        className="btn btn-ghost h-6"
        onClick={() => {
          writeStored(KEPT_KEY, [...readStored<string[]>(KEPT_KEY, []), session.id])
          setKept(true)
        }}
      >
        Keep
      </button>
    </div>
  )
}
