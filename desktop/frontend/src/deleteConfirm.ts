// deleteConfirmText is the question the delete confirm asks: the thread and its chat go, and its branch stays.
export function deleteConfirmText(title: string, branch: string): string {
  return `Delete ${title}? Its chat goes too.${branch ? ` The branch ${branch} stays.` : ''}`
}

// deletesWorktree says whether confirming takes the worktree too: only one the thread has to itself, and only when the
// owner ticked the box. offered is "" when there is none, and null until the app has said.
export function deletesWorktree(offered: string | null, ticked: boolean): boolean {
  return ticked && Boolean(offered)
}
