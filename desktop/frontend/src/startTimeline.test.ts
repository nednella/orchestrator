import assert from 'node:assert/strict'
import { test } from 'node:test'
import { startTimeline } from './startTimeline.ts'

const starting = { id: 'shop/abcd2345', createdAt: 1000, start: 'starting' as const, startError: '', detail: 'Setting up the worktree', firstMessage: 'Fix the login form', startQueue: [] as string[] }

test('a thread that starts shows its message as sent, the turn working and the set-up step quietly', () => {
  const { timeline } = startTimeline(starting)
  assert.deepEqual(
    timeline.items.map((i) => i.kind),
    ['user', 'working', 'thinking'],
  )
  const [user, , step] = timeline.items
  assert.equal(user.kind === 'user' && user.text, 'Fix the login form')
  assert.equal(step.kind === 'thinking' && step.label, 'Setting up the worktree')
  assert.equal(timeline.turnOpen, true)
})

test('messages sent while it starts wait in the queue', () => {
  const { timeline } = startTimeline({ ...starting, startQueue: ['and the signup form'] })
  assert.deepEqual(timeline.queued.map((q) => q.text), ['and the signup form'])
})

test('a failed start ends the turn with the reason, the message still shown as sent', () => {
  const { timeline } = startTimeline({ ...starting, start: 'failed', startError: 'the set-up failed', detail: 'the set-up failed' })
  assert.deepEqual(
    timeline.items.map((i) => i.kind),
    ['user', 'startFailed'],
  )
  assert.equal(timeline.turnOpen, false)
})

test('a start with no message has the step alone', () => {
  const { timeline } = startTimeline({ ...starting, firstMessage: '', detail: '' })
  assert.deepEqual(
    timeline.items.map((i) => i.kind),
    ['thinking'],
  )
})

test('a cancelled start keeps the first message and the held ones, ends the timer of the working row and offers the retry', () => {
  const { timeline } = startTimeline({ ...starting, start: 'cancelled', startError: 'Start cancelled', detail: 'Start cancelled', startQueue: ['and the signup form'] })
  assert.deepEqual(
    timeline.items.map((i) => i.kind),
    ['user', 'startFailed'],
  )
  const failed = timeline.items[timeline.items.length - 1]
  assert.equal(failed.kind === 'startFailed' && failed.cancelled, true)
  assert.equal(failed.kind === 'startFailed' && failed.text, 'Start cancelled')
  assert.deepEqual(timeline.queued.map((q) => q.text), ['and the signup form'])
  assert.equal(timeline.turnOpen, false)
})

test('a failed start is not a cancelled one', () => {
  const { timeline } = startTimeline({ ...starting, start: 'failed', startError: 'no room for a session' })
  const failed = timeline.items.at(-1)
  assert.equal(failed?.kind === 'startFailed' && failed.cancelled, false)
})
