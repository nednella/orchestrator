import assert from 'node:assert/strict'
import { test } from 'node:test'
import { cleanupConfirmText, cleanupForced } from './cleanupConfirm.ts'

test('only a kept clean-up is forced', () => {
  assert.equal(cleanupForced('blocked'), true)
  assert.equal(cleanupForced('ask'), false)
  assert.equal(cleanupForced(''), false)
  assert.equal(cleanupForced('pending'), false)
})

test('confirming an ask names what goes and keeps the branch', () => {
  const text = cleanupConfirmText({ cleanup: 'ask', cleanupReason: '', worktree: '/repo/trees/issue-7', branch: 'issue-7' })
  assert.equal(text, "Remove the worktree trees/issue-7 and the thread's temp files? The branch issue-7 stays. A worktree that still holds work is kept.")
})

test('forcing names why the worktree was kept', () => {
  const text = cleanupConfirmText({ cleanup: 'blocked', cleanupReason: 'a shell is open in this worktree', worktree: '/repo/trees/issue-7', branch: 'issue-7' })
  assert.equal(text, 'Remove the worktree trees/issue-7 anyway? It was kept: a shell is open in this worktree. What only it holds is lost. The branch issue-7 stays.')
})

test('a thread with no worktree loses only its temp files', () => {
  assert.equal(cleanupConfirmText({ cleanup: 'ask', cleanupReason: '', worktree: '', branch: '' }), "Remove the thread's temp files?")
  assert.equal(cleanupConfirmText({ cleanup: 'blocked', cleanupReason: 'settling the thread failed: x', worktree: '', branch: 'b' }), 'Clean up anyway? It was kept: settling the thread failed: x. What only it holds is lost. The branch b stays.')
})
