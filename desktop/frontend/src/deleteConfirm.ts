import type { DeletePlan } from './types'

// deleteConfirmText is the question the delete confirm asks: the thread and its chat go, and its branch stays.
export function deleteConfirmText(title: string, branch: string): string {
  return `Delete ${title}? Its chat goes too.${branch ? ` The branch ${branch} stays.` : ''}`
}

// childrenText says which asleep child threads go with the thread: every title up to three, else three and how many more.
export function childrenText(titles: string[]): string {
  if (titles.length === 0) return ''
  if (titles.length === 1) return `Its asleep child thread ${titles[0]} goes too.`
  const named = titles.length > 3 ? [...titles.slice(0, 3), `${titles.length - 3} more`] : titles
  return `Its ${titles.length} asleep child threads go too: ${named.slice(0, -1).join(', ')} and ${named[named.length - 1]}.`
}

// worktreesLabel is the box that takes the worktrees the delete may take with the thread.
export function worktreesLabel(worktrees: number): string {
  return worktrees === 1 ? 'Delete the worktree too' : `Delete ${worktrees} worktrees too`
}

// deletesWorktree says whether confirming takes the worktrees: only ones the thread and its descendants have to
// themselves, and only when the owner ticked the box. plan is null until the app has said.
export function deletesWorktree(plan: DeletePlan | null, ticked: boolean): boolean {
  return ticked && !!plan && plan.worktrees > 0
}
