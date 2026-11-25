import assert from 'node:assert/strict'
import { test } from 'node:test'
import { deleteConfirmText, deletesWorktree } from './deleteConfirm.ts'

test('the confirm says the chat goes and the branch stays', () => {
  assert.equal(deleteConfirmText('#7 work', 'issue-7'), 'Delete #7 work? Its chat goes too. The branch issue-7 stays.')
})

test('a thread with no branch names none', () => {
  assert.equal(deleteConfirmText('Look around', ''), 'Delete Look around? Its chat goes too.')
})

test('the worktree goes only when it is offered and ticked', () => {
  assert.equal(deletesWorktree('/repo/trees/issue-7', true), true)
  assert.equal(deletesWorktree('/repo/trees/issue-7', false), false)
  assert.equal(deletesWorktree('', true), false)
  assert.equal(deletesWorktree(null, true), false)
})
