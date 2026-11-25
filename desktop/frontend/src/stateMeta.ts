import type { State } from './types'

export const STATE_LABEL: Record<State, string> = {
  waiting: 'Attention',
  working: 'Working',
  idle: 'Idle',
  settled: 'Settled',
}

export function stateLabel(state: State, chat: boolean): string {
  return chat && state === 'working' ? 'Thinking' : STATE_LABEL[state]
}
