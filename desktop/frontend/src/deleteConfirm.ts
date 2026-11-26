import type { DeletePlan } from './types'

// deleteConfirmText is the question the delete confirm asks: the thread and its chat go, and its branch stays.
export function deleteConfirmText(title: string, branch: string): string {
  return `Delete ${title}? Its chat goes too.${branch ? ` The branch ${branch} stays.` : ''}`
}

// worktreesLabel is the box that takes the worktrees the delete may take with the thread.
export function worktreesLabel(worktrees: number): string {
  return worktrees === 1 ? 'Delete the worktree too' : `Delete ${worktrees} worktrees too`
}

// deletesWorktree says whether confirming takes the worktrees: only ones the thread and its descendants have to
// themselves, when the owner ticked the box or when a descendant's must go with it. plan is null until the app has said.
export function deletesWorktree(plan: DeletePlan | null, ticked: boolean): boolean {
  if (!plan || plan.worktrees === 0) return false
  return plan.required || ticked
}
