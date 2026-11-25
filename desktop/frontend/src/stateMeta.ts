import type { State } from './types'

export const STATE_LABEL: Record<State, string> = {
  waiting: 'Attention',
  working: 'Working',
  idle: 'Idle',
  settled: 'Settled',
}
