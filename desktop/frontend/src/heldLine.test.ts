import assert from 'node:assert/strict'
import { test } from 'node:test'
import { buildTimeline } from './chatTimeline.ts'
import { withHeldError } from './heldLine.ts'

const timeline = buildTimeline([{ seq: 1, at: 1, kind: 'user', id: 'u', text: 'Fix it' }], {})

test('a running thread with no held error keeps its chat as it is', () => {
  assert.equal(withHeldError(timeline, { start: '', startError: '', lastEventAt: 5 }), timeline)
})

test('a running thread whose held messages did not go says why, in red, after the chat', () => {
  const last = withHeldError(timeline, { start: '', startError: 'Thread is restarting', lastEventAt: 5 }).items.at(-1)
  assert.deepEqual(last, { kind: 'notice', key: 'held-error', at: 5, text: 'Held messages not sent: Thread is restarting', error: true })
})

test('a thread that still shows its start leaves the reason to the start view', () => {
  assert.equal(withHeldError(timeline, { start: 'failed', startError: 'x', lastEventAt: 5 }), timeline)
})
