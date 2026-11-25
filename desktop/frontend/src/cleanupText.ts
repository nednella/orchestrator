import type { Cleanup } from './types'

// keptLine says why a clean-up kept the worktree.
export function keptLine(reason: string): string {
  return `Kept: ${reason}`
}

// cleanupLine is what one clean-up did: what it removed, or why the checks kept the worktree.
export function cleanupLine(c: Cleanup): string {
  return c.status === 'blocked' ? keptLine(c.reason) : `Removed ${c.removed.join(', ')}`
}
