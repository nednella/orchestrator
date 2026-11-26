import { useEffect, useState } from 'react'
import { useAgentos } from '../AgentosContext'
import { api } from '../api'
import { deleteConfirmText, deletesWorktree, worktreesLabel } from '../deleteConfirm'
import type { DeletePlan, Session } from '../types'
import { ConfirmRow } from './ConfirmRow'

type DeleteConfirmProps = { session: Session; onClose(): void }

// DeleteConfirm asks before the thread and its chat go. It offers the worktrees agentos made for the thread and its
// descendants that no other thread or shell works in, unticked; a descendant's cannot stay, so then the box is ticked
// and fixed.
export function DeleteConfirm({ session, onClose }: DeleteConfirmProps) {
  const { deleteThread, report } = useAgentos('deleteThread', 'report')
  const [plan, setPlan] = useState<DeletePlan | null>(null)
  const [ticked, setTicked] = useState(false)

  useEffect(() => {
    let current = true
    api.deletePlan(session.id).then(
      (p) => current && setPlan(p),
      () => current && setPlan({ worktrees: 0, required: false }),
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
          {plan && plan.worktrees > 0 && (
            <label className="flex cursor-pointer items-center gap-2 text-soft">
              <input type="checkbox" checked={plan.required || ticked} disabled={plan.required} onChange={(e) => setTicked(e.target.checked)} />
              {worktreesLabel(plan.worktrees)}
            </label>
          )}
          {plan?.required && <span className="text-soft">A child thread's worktree cannot stay once its thread goes.</span>}
        </span>
      }
      confirmLabel="Delete"
      onCancel={onClose}
      onConfirm={() => {
        onClose()
        report(() => deleteThread(session.id, deletesWorktree(plan, ticked)))
      }}
    />
  )
}
