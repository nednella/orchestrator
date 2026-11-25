import type { Session } from './types'

type CleanupTarget = Pick<Session, 'cleanup' | 'cleanupReason' | 'worktree' | 'branch'>

// cleanupForced says whether confirming the clean-up forces it. Only one the checks kept, with a reason, is forced:
// confirming an ask runs every check, so a worktree that still holds work stays.
export function cleanupForced(cleanup: Session['cleanup']): boolean {
  return cleanup === 'blocked'
}

function tail(path: string): string {
  return path.split('/').slice(-2).join('/')
}

// cleanupConfirmText is the question the confirm asks: what goes, and for a forced clean-up why it was kept, which is
// what forcing loses. The branch always stays.
export function cleanupConfirmText({ cleanup, cleanupReason, worktree, branch }: CleanupTarget): string {
  const stays = branch ? ` The branch ${branch} stays.` : ''
  if (cleanupForced(cleanup)) {
    const what = worktree ? `Remove the worktree ${tail(worktree)} anyway?` : 'Clean up anyway?'
    return `${what} It was kept: ${cleanupReason}. What only it holds is lost.${stays}`
  }
  const what = worktree ? `the worktree ${tail(worktree)} and the thread's temp files` : "the thread's temp files"
  return `Remove ${what}?${stays}${worktree ? ' A worktree that still holds work is kept.' : ''}`
}
