import type { Timeline } from './chatTimeline.ts'
import type { Thread } from './types.ts'

// withSetup adds the line of the project's set-up command to the end of a chat: quiet while it runs, and with its exit
// code and terminal when it failed. It leaves the thread's turn as it was: the agent works while the command runs.
export function withSetup(timeline: Timeline, thread: Pick<Thread, 'setup' | 'setupExit' | 'setupShell' | 'setupProblem'>): Timeline {
  if (thread.setup === '') return timeline
  const failed = thread.setup === 'failed'
  const exited = thread.setupProblem ? `The set-up command did not start: ${thread.setupProblem}` : `The set-up command exited with code ${thread.setupExit}`
  const text = failed ? exited : 'Running the set-up command'
  return { ...timeline, items: [...timeline.items, { kind: 'setup', key: 'setup', text, failed, shell: thread.setupShell }] }
}
