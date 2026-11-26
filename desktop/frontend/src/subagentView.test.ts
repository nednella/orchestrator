import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { AgentItem, WorkStep } from './chatTimeline.ts'
import { closedIn, heldBy, keptFor, subagentStatus, viewedIn } from './subagentView.ts'

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

const view = { thread: 'p/a', agent: 'a1' }

test('a view shows only in its own thread', () => {
  assert.equal(viewedIn(view, 'p/a'), 'a1')
  assert.equal(viewedIn(view, 'p/b'), null)
  assert.equal(viewedIn(null, 'p/a'), null)
})

test('closing a thread leaves the view of another', () => {
  assert.equal(closedIn(view, 'p/a'), null)
  assert.equal(closedIn(view, 'p/b'), view)
})

test('a view of a thread not on screen closes', () => {
  assert.equal(keptFor(view, 'p/a'), view)
  assert.equal(keptFor(view, 'p/b'), null, 'the owner left the thread, or it opened for a thread not shown')
  assert.equal(keptFor(null, 'p/a'), null)
})

test('a view whose subagent the loaded chat does not hold closes', () => {
  assert.equal(heldBy(view, 'p/a', true, true), view)
  assert.equal(heldBy(view, 'p/a', true, false), null)
  assert.equal(heldBy(view, 'p/a', false, false), view, 'a chat that loads may not hold it yet')
  assert.equal(heldBy(view, 'p/b', true, false), view, "another thread's chat says nothing of it")
})
