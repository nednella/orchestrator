import assert from 'node:assert/strict'
import { test } from 'node:test'
import { childrenText, deleteConfirmText, deletesWorktree, worktreesLabel } from './deleteConfirm.ts'

test('the confirm says the chat goes and the branch stays', () => {
  assert.equal(deleteConfirmText('#7 work', 'issue-7'), 'Delete #7 work? Its chat goes too. The branch issue-7 stays.')
})

test('a thread with no branch names none', () => {
  assert.equal(deleteConfirmText('Look around', ''), 'Delete Look around? Its chat goes too.')
})

test('the box says how many worktrees go', () => {
  assert.equal(worktreesLabel(1), 'Delete the worktree too')
  assert.equal(worktreesLabel(3), 'Delete 3 worktrees too')
})

test('the worktrees go only when there are some and the box is ticked or they must', () => {
  assert.equal(deletesWorktree({ worktrees: 1, required: false, children: [] }, true), true)
  assert.equal(deletesWorktree({ worktrees: 1, required: false, children: [] }, false), false)
  assert.equal(deletesWorktree({ worktrees: 2, required: true, children: ['Builder'] }, false), true)
  assert.equal(deletesWorktree({ worktrees: 0, required: false, children: [] }, true), false)
  assert.equal(deletesWorktree(null, true), false)
})

test('the confirm names the asleep child threads that go, up to three', () => {
  assert.equal(childrenText([]), '')
  assert.equal(childrenText(['Builder']), 'Its asleep child thread Builder goes too.')
  assert.equal(childrenText(['Builder', 'Tester']), 'Its 2 asleep child threads go too: Builder and Tester.')
  assert.equal(childrenText(['A', 'B', 'C']), 'Its 3 asleep child threads go too: A, B and C.')
  assert.equal(childrenText(['A', 'B', 'C', 'D', 'E']), 'Its 5 asleep child threads go too: A, B, C and 2 more.')
})
