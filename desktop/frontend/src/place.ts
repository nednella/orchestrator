import type { Access, Place } from './types'

export const PLACE_LABEL: Record<Place, string> = {
  worktree: 'New worktree',
  project: 'Current branch',
}

export const PLACE_HINT: Record<Place, string> = {
  worktree: 'It works on its own branch in a new worktree, so your checkout stays as it is',
  project: 'It runs in the project folder, on the branch checked out there',
}

export const PLACE_ISSUE_HINT = 'An issue thread runs in its own worktree'

// placeFor is where a new thread starts: an issue thread in a worktree, any other where the setting says, a new
// worktree when it is unset.
export function placeFor(setting: Place | '', issue: boolean): Place {
  return issue ? 'worktree' : setting || 'worktree'
}

// defaultPlace is where a thread runs when the owner does not pick: an issue and a Write thread get a worktree, a Read
// thread with no issue the project folder.
export function defaultPlace(access: Access, issue: boolean): Place {
  return issue || access !== 'read' ? 'worktree' : 'project'
}
