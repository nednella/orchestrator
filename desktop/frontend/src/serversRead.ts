import type { State } from './types'

export const SERVERS_POLL_MS = 5000

// pollsServers says whether a thread's servers are read every few seconds: only while their tab is the one shown and
// the thread runs. Otherwise they are read when the details open and at each turn end, which is when a thread starts
// or stops a server.
export function pollsServers(shown: boolean, state: State | undefined): boolean {
  return shown && state !== undefined && state !== 'settled'
}

// turnEnded says whether a thread's change of state from prev to next ends a turn.
export function turnEnded(prev: State | undefined, next: State | undefined): boolean {
  return prev === 'working' && next !== undefined && next !== 'working'
}
