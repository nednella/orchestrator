import type { ConfirmRequest } from './confirmQueue'
import type { Session } from './types'

type CleanupTarget = Pick<Session, 'cleanup' | 'cleanupReason' | 'worktree' | 'branch'>

// cleanupForced says whether confirming the clean-up forces it. Only one the checks kept, with a reason, is forced:
// confirming an ask runs every check, so a worktree that still holds work stays.
export function cleanupForced(cleanup: Session['cleanup']): boolean {
  return cleanup === 'blocked'
}

// cleanupNowShown says whether the thread menu offers Clean up now, a clean-up with every check: for every kept one,
// whatever kept it, and for a thread whose worktree agentos made when no clean-up runs or asks.
export function cleanupNowShown({ cleanup, worktree }: Pick<Session, 'cleanup' | 'worktree'>): boolean {
  return cleanup === 'blocked' || (cleanup === '' && worktree !== '')
}

const PASSING = ['a turn is running', 'messages wait for this thread', 'a shell is open in this worktree', 'a shell whose folder is unknown is open']

// passingReason says whether the clean-up was kept for a use of the worktree that ends by itself, a turn, held
// messages, another thread or a shell, so a clean-up that runs every check again may go through later.
export function passingReason(reason: string): boolean {
  return PASSING.includes(reason) || /^Thread .+ also works in this worktree$/.test(reason)
}

function tail(path: string): string {
  return path.split('/').slice(-2).join('/')
}

// cleanupRequest is the confirm of a clean-up: what goes, and for a forced clean-up why it was kept, which is what
// forcing loses. The branch always stays.
export function cleanupRequest({ cleanup, cleanupReason, worktree, branch }: CleanupTarget): Omit<ConfirmRequest, 'id'> {
  const stays = branch ? `The branch ${branch} stays.` : ''
  if (cleanupForced(cleanup)) {
    const title = worktree ? `Remove the worktree ${tail(worktree)} anyway?` : 'Clean up anyway?'
    return { title, body: [[`It was kept: ${cleanupReason}. What only it holds is lost.`, stays].filter(Boolean).join(' ')], confirm: 'Remove', tone: 'danger' }
  }
  const what = worktree ? `the worktree ${tail(worktree)} and the thread's temp files` : "the thread's temp files"
  const body = [stays, worktree ? 'A worktree that still holds work is kept.' : ''].filter(Boolean).join(' ')
  return { title: `Remove ${what}?`, ...(body ? { body: [body] } : {}), confirm: 'Remove', tone: 'danger' }
}
