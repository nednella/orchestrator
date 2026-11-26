import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { AgentItem, WorkStep } from './chatTimeline.ts'
import { subagentStatus } from './subagentView.ts'

const note: WorkStep = { kind: 'note', key: 'note-1', text: 'Looking.' }

const agent = (over: Partial<AgentItem>): AgentItem => ({
  kind: 'agent',
  key: 'agent-a1',
  id: 'a1',
  turn: 'u1',
  description: 'Find the callers',
  type: '',
  prompt: 'Find every caller of redirect().',
  model: '',
  startedAt: 10_000,
  endedAt: null,
  state: 'running',
  tools: 0,
  steps: [],
  result: null,
  ...over,
})

test('starting before a step', () => {
  assert.equal(subagentStatus(agent({}), 75_000), 'Starting')
})

test('running ticks', () => {
  const running = agent({ steps: [note] })
  assert.equal(subagentStatus(running, 22_400), 'Running 12s')
  assert.equal(subagentStatus(running, 72_000), 'Running 1m 02s')
  assert.equal(subagentStatus(running, 10_000 + 3_725_000), 'Running 1h 02m')
  assert.equal(subagentStatus(running, 9_000), 'Running 0s', 'a clock behind the start reads no time')
})

test('completed in', () => {
  const done = agent({ steps: [note], state: 'done', endedAt: 200_000 })
  assert.equal(subagentStatus(done, 900_000), 'Completed in 3m 10s', 'the time stops at the end')
  assert.equal(subagentStatus({ ...done, steps: [] }, 900_000), 'Completed in 3m 10s', 'one that ran no step ended all the same')
})

test('failed and stopped', () => {
  assert.equal(subagentStatus(agent({ state: 'failed', endedAt: 20_000 }), 30_000), 'Failed')
  assert.equal(subagentStatus(agent({ state: 'stopped', endedAt: 20_000, steps: [note] }), 30_000), 'Stopped')
})
