import assert from 'node:assert/strict'
import { test } from 'node:test'
import { deleteRequest } from './deleteThread.ts'

const asleep = { title: '#7 work', branch: 'issue-7', state: 'settled' as const }
const none = { worktrees: 0, children: [] }

test('the body names the branch that stays', () => {
  const r = deleteRequest(asleep, none)
  assert.equal(r.title, 'Delete thread "#7 work"?')
  assert.deepEqual(r.body, ['Its conversation goes, and so does its row. This cannot be undone. The branch issue-7 stays.'])
  assert.equal(r.confirm, 'Delete')
  assert.equal(r.tone, 'danger')
})

test('no branch sentence without a branch', () => {
  assert.deepEqual(deleteRequest({ ...asleep, branch: '' }, none).body, ['Its conversation goes, and so does its row. This cannot be undone.'])
})

test('a live thread is stopped first', () => {
  assert.deepEqual(deleteRequest({ ...asleep, state: 'idle' }, none).body, ['This stops the thread first. Its conversation goes, and so does its row. This cannot be undone. The branch issue-7 stays.'])
})

test('asleep children are listed', () => {
  const r = deleteRequest(asleep, { worktrees: 0, children: ['Builder', 'Tester'] })
  assert.equal(r.body?.at(-1), 'Its asleep children go too:')
  assert.deepEqual(r.list, ['Builder', 'Tester'])
  assert.equal(deleteRequest(asleep, none).list, undefined)
})

test('the box shows only when worktrees may go, unticked', () => {
  assert.equal(deleteRequest(asleep, none).checkbox, undefined)
  assert.deepEqual(deleteRequest(asleep, { worktrees: 1, children: [] }).checkbox, { label: 'Also delete its worktree', checked: false })
  assert.deepEqual(deleteRequest(asleep, { worktrees: 3, children: [] }).checkbox, { label: 'Also delete its 3 worktrees', checked: false })
})
