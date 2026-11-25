import { useState } from 'react'
import { useAgentos } from '../AgentosContext'
import { cleanupConfirmText, cleanupForced } from '../cleanupConfirm'
import { readStored, writeStored } from '../storage'
import { useArmedConfirm } from '../useArmedConfirm'
import type { ArmedConfirm } from '../useArmedConfirm'
import { ConfirmRow } from './ConfirmRow'
import type { Session } from '../types'

// remove is the confirm of the removal, for a caller that also arms it from elsewhere, such as a menu.
type CleanupControlsProps = { session: Session; remove?: ArmedConfirm<true> }

const KEPT_KEY = 'agentos.keptCleanups'

export function CleanupControls({ session, remove: given }: CleanupControlsProps) {
  const { cleanupSession, report } = useAgentos('cleanupSession', 'report')
  const own = useArmedConfirm()
  const remove = given ?? own
  const [kept, setKept] = useState(() => readStored<string[]>(KEPT_KEY, []).includes(session.id))
  const { cleanup, cleanupReason } = session

  if (cleanup === 'pending') return <span className="text-small text-dim">cleaning up…</span>

  if (remove.armed) {
    return (
      <ConfirmRow
        danger
        stacked
        message={cleanupConfirmText(session)}
        confirmLabel="Remove"
        onCancel={remove.disarm}
        onConfirm={() => {
          remove.disarm()
          report(() => cleanupSession(session.id, cleanupForced(cleanup)))
        }}
      />
    )
  }

  if (cleanup === 'ask' && kept) return null

  if (cleanup === 'blocked') {
    return (
      <div className="flex w-full flex-wrap items-center gap-x-2 gap-y-1">
        <span className="min-w-0 text-small text-danger">Not cleaned up: {cleanupReason}</span>
        <button className="btn h-6" onClick={() => remove.arm()}>
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
      <button className="btn h-6" onClick={() => remove.arm()}>
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
