import type { Place } from './types'

export const PLACE_LABEL: Record<Place, string> = {
  worktree: 'New worktree',
  project: 'Current checkout',
}

// placeFor is where a new thread starts: an issue thread in a worktree, any other where the setting says, a new
// worktree when it is unset.
export function placeFor(setting: Place | '', issue: boolean): Place {
  return issue ? 'worktree' : setting || 'worktree'
}
