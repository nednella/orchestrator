import assert from 'node:assert/strict'
import { test } from 'node:test'
import { startStatus } from './threadStart.ts'

test('a thread that runs has no start to show', () => {
  assert.deepEqual(startStatus({ start: '', startError: '' }), { kind: 'none' })
})

test('a thread that starts says so', () => {
  assert.deepEqual(startStatus({ start: 'starting', startError: '' }), { kind: 'starting', text: 'Starting the thread' })
})

test('a failed start says why, and still says it when the reason is missing', () => {
  assert.deepEqual(startStatus({ start: 'failed', startError: 'worktree_command failed for fix-login' }), { kind: 'failed', text: 'worktree_command failed for fix-login' })
  assert.deepEqual(startStatus({ start: 'failed', startError: '' }), { kind: 'failed', text: 'The thread did not start' })
})
