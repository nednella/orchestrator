import type { AgentState } from './chatTimeline'
import type { State } from './types'

export const STATE_LABEL: Record<State, string> = {
  waiting: 'Attention',
  working: 'Working',
  idle: 'Idle',
  settled: 'Sleeping',
}

export const AGENT_STATE_LABEL: Record<AgentState, string> = { running: 'Running', done: 'Done', failed: 'Failed', stopped: 'Stopped' }
