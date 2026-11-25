import assert from 'node:assert/strict'
import { test } from 'node:test'
import { splitBranch } from './branchLabel.ts'

test('a short name stays whole', () => {
  assert.deepEqual(splitBranch('main'), { head: 'main', tail: '' })
  assert.deepEqual(splitBranch('issue-574-lay'), { head: 'issue-574-lay', tail: '' })
})

test('a long name keeps its last characters apart from the rest', () => {
  assert.deepEqual(splitBranch('issue-574-sidebar-layout'), { head: 'issue-574-sideba', tail: 'r-layout' })
})

test('the two parts make the name again', () => {
  const name = 'feature/very-long-branch-name-here'
  const { head, tail } = splitBranch(name)
  assert.equal(head + tail, name)
})
