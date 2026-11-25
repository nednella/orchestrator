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

test('an empty name is one empty part', () => {
  assert.deepEqual(splitBranch(''), { head: '', tail: '' })
})

test('a name at the threshold stays whole and one more character splits', () => {
  assert.deepEqual(splitBranch('a'.repeat(16)), { head: 'a'.repeat(16), tail: '' })
  assert.deepEqual(splitBranch('a'.repeat(17)), { head: 'a'.repeat(9), tail: 'a'.repeat(8) })
})

test('an emoji at the tail boundary is not cut in two', () => {
  const name = `${'a'.repeat(10)}😀${'b'.repeat(7)}`
  assert.deepEqual(splitBranch(name), { head: 'a'.repeat(10), tail: `😀${'b'.repeat(7)}` })
})

test('a non-Latin name splits by characters', () => {
  const name = '功能分支名称很长很长很长很长很长很长很长'
  const { head, tail } = splitBranch(name)
  assert.equal(Array.from(tail).length, 8)
  assert.equal(head + tail, name)
})
