import type { Thread } from './types'

// StartStatus is what a thread's chat says while its agent has not started: nothing once it runs, 'starting' with the
// step the owner waits on, or 'failed' with why, which a retry goes on from.
export type StartStatus = { kind: 'none' } | { kind: 'starting'; text: string } | { kind: 'failed'; text: string }

export function startStatus(thread: Pick<Thread, 'start' | 'startError'>): StartStatus {
  switch (thread.start) {
    case 'starting':
      return { kind: 'starting', text: 'Starting the thread' }
    case 'failed':
      return { kind: 'failed', text: thread.startError || 'The thread did not start' }
    default:
      return { kind: 'none' }
  }
}
