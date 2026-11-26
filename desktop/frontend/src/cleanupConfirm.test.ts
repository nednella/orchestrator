import assert from 'node:assert/strict'
import { test } from 'node:test'
import { cleanupForced, cleanupRequest, cleanupNowShown } from './cleanupConfirm.ts'

test('only a kept clean-up is forced', () => {
  assert.equal(cleanupForced('blocked'), true)
  assert.equal(cleanupForced('ask'), false)
  assert.equal(cleanupForced(''), false)
  assert.equal(cleanupForced('pending'), false)
})

test('confirming an ask names what goes and keeps the branch', () => {
  const r = cleanupRequest({ cleanup: 'ask', cleanupReason: '', worktree: '/repo/trees/issue-7', branch: 'issue-7' })
  assert.equal(r.title, "Remove the worktree trees/issue-7 and the thread's temp files?")
  assert.deepEqual(r.body, ['The branch issue-7 stays. A worktree that still holds work is kept.'])
  assert.equal(r.confirm, 'Remove')
  assert.equal(r.tone, 'danger')
})

test('forcing names why the worktree was kept', () => {
  const r = cleanupRequest({ cleanup: 'blocked', cleanupReason: 'a shell is open in this worktree', worktree: '/repo/trees/issue-7', branch: 'issue-7' })
  assert.equal(r.title, 'Remove the worktree trees/issue-7 anyway?')
  assert.deepEqual(r.body, ['It was kept: a shell is open in this worktree. What only it holds is lost. The branch issue-7 stays.'])
})

test('a thread with no worktree loses only its temp files', () => {
  const r = cleanupRequest({ cleanup: 'ask', cleanupReason: '', worktree: '', branch: '' })
  assert.equal(r.title, "Remove the thread's temp files?")
  assert.equal(r.body, undefined)
  const forced = cleanupRequest({ cleanup: 'blocked', cleanupReason: 'settling the thread failed: x', worktree: '', branch: 'b' })
  assert.equal(forced.title, 'Clean up anyway?')
  assert.deepEqual(forced.body, ['It was kept: settling the thread failed: x. What only it holds is lost. The branch b stays.'])
})

test('every kept clean-up offers Clean up now, whatever kept it', () => {
  assert.equal(cleanupNowShown({ cleanup: 'blocked', worktree: '/repo/trees/issue-7' }), true)
  assert.equal(cleanupNowShown({ cleanup: 'blocked', worktree: '' }), true)
})

test('Clean up now waits while a clean-up runs or asks, and needs a worktree orchestrator made otherwise', () => {
  assert.equal(cleanupNowShown({ cleanup: '', worktree: '/repo/trees/issue-7' }), true)
  assert.equal(cleanupNowShown({ cleanup: '', worktree: '' }), false)
  assert.equal(cleanupNowShown({ cleanup: 'pending', worktree: '/repo/trees/issue-7' }), false)
  assert.equal(cleanupNowShown({ cleanup: 'ask', worktree: '/repo/trees/issue-7' }), false)
})
