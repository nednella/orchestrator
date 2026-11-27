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
  const { timeline } = startTimeline({ ...starting, start: 'failed', startError: 'worktree_command failed', detail: 'worktree_command failed' })
  assert.deepEqual(
    timeline.items.map((i) => i.kind),
    ['user', 'working', 'startFailed'],
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
