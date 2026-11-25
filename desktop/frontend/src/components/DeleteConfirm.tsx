import { useEffect, useState } from 'react'
import { useAgentos } from '../AgentosContext'
import { api } from '../api'
import { deleteConfirmText, deletesWorktree } from '../deleteConfirm'
import type { Session } from '../types'
import { ConfirmRow } from './ConfirmRow'

type DeleteConfirmProps = { session: Session; onClose(): void }

// DeleteConfirm asks before the thread and its chat go. It offers the worktree agentos made for the thread only when no
// other thread or shell works in it, unticked.
export function DeleteConfirm({ session, onClose }: DeleteConfirmProps) {
  const { deleteThread, report } = useAgentos('deleteThread', 'report')
  const [offered, setOffered] = useState<string | null>(null)
  const [ticked, setTicked] = useState(false)

  useEffect(() => {
    let current = true
    api.ownWorktree(session.id).then(
      (worktree) => current && setOffered(worktree),
      () => current && setOffered(''),
    )
    return () => {
      current = false
    }
  }, [session.id])

  return (
    <ConfirmRow
      danger
      stacked
      label="Delete thread"
      message={
        <span className="flex flex-col gap-1.5">
          <span>{deleteConfirmText(session.title, session.branch)}</span>
          {offered && (
            <label className="flex cursor-pointer items-center gap-2 text-soft">
              <input type="checkbox" checked={ticked} onChange={(e) => setTicked(e.target.checked)} />
              Delete the worktree too
            </label>
          )}
        </span>
      }
      confirmLabel="Delete"
      onCancel={onClose}
      onConfirm={() => {
        onClose()
        report(() => deleteThread(session.id, deletesWorktree(offered, ticked)))
      }}
    />
  )
}
