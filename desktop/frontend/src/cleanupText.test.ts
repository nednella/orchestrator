import assert from 'node:assert/strict'
import { test } from 'node:test'
import { cleanupLine } from './cleanupText.ts'
import type { Cleanup } from './types'

const entry = (c: Partial<Cleanup>): Cleanup => ({ at: 1, sessionTitle: '#7 work', issue: 7, pr: 12, merged: true, status: 'done', removed: [], reason: '', ...c })

test('removed line names the worktree', () => {
  const c = entry({ removed: ['worktree /repo/trees/issue-7', 'thread put to sleep', 'temp folder'] })
  assert.equal(cleanupLine(c), 'Removed worktree /repo/trees/issue-7, thread put to sleep, temp folder')
})

test('kept line gives the reason', () => {
  const c = entry({ status: 'blocked', reason: 'the worktree of issue-7 has uncommitted changes' })
  assert.equal(cleanupLine(c), 'Kept: the worktree of issue-7 has uncommitted changes')
})
