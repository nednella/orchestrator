import { useEffect, useState } from 'react'
import { useAgentos } from '../AgentosContext'
import { api } from '../api'
import { childrenText, deleteConfirmText, deletesWorktree, worktreesLabel } from '../deleteConfirm'
import type { DeletePlan, Session } from '../types'
import { ConfirmRow } from './ConfirmRow'

type DeleteConfirmProps = { session: Session; onClose(): void }

// DeleteConfirm asks before the thread and its chat go, and names the asleep child threads that go with it. It offers
// the worktrees agentos made for the thread and its descendants that no other thread or shell works in, unticked.
export function DeleteConfirm({ session, onClose }: DeleteConfirmProps) {
  const { deleteThread, report } = useAgentos('deleteThread', 'report')
  const [plan, setPlan] = useState<DeletePlan | null>(null)
  const [ticked, setTicked] = useState(false)

  useEffect(() => {
    let current = true
    api.deletePlan(session.id).then(
      (p) => current && setPlan(p),
      () => current && setPlan({ worktrees: 0, children: [] }),
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
          {plan && plan.children.length > 0 && <span>{childrenText(plan.children)}</span>}
          {plan && plan.worktrees > 0 && (
            <label className="flex cursor-pointer items-center gap-2 text-soft">
              <input type="checkbox" checked={ticked} onChange={(e) => setTicked(e.target.checked)} />
              {worktreesLabel(plan.worktrees)}
            </label>
          )}
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
