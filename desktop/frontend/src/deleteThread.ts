import type { ConfirmRequest } from './confirmQueue'
import type { DeletePlan, Session } from './types'

// deleteRequest is the confirm of a thread's delete: a live thread is stopped first, its conversation and row go, its
// branch stays, its asleep descendants go with it, and the box offers the worktrees the delete may take, unticked.
export function deleteRequest({ title, branch, state }: Pick<Session, 'title' | 'branch' | 'state'>, plan: DeletePlan): Omit<ConfirmRequest, 'id'> {
  const what = [state === 'settled' ? '' : 'This stops the thread first.', 'Its conversation goes, and so does its row. This cannot be undone.', branch ? `The branch ${branch} stays.` : '']
  const children = plan.children.length > 0
  return {
    title: `Delete thread "${title}"?`,
    body: [what.filter(Boolean).join(' '), ...(children ? ['Its asleep children go too:'] : [])],
    ...(children ? { list: plan.children } : {}),
    ...(plan.worktrees > 0 ? { checkbox: { label: plan.worktrees === 1 ? 'Also delete its worktree' : `Also delete its ${plan.worktrees} worktrees`, checked: false } } : {}),
    confirm: 'Delete',
    tone: 'danger',
  }
}
