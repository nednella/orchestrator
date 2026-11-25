import type { State } from './types'

// watchAgain says whether a thread's chat must be watched again: a settled thread cannot be watched, so a chat opened
// on one watches once it is woken.
export function watchAgain(before: State | undefined, now: State): boolean {
  return before === 'settled' && now !== 'settled'
}
