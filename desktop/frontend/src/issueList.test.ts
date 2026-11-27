import assert from 'node:assert/strict'
import { test } from 'node:test'
import { DEFAULT_ISSUE_PREFS, issueFilterCount, issueFacets, listIssues } from './issueList.ts'
import type { Issue } from './types.ts'

const issue = (number: number, extra: Partial<Issue> = {}): Issue => ({
  number, title: `Issue ${number}`, type: 'bug', section: 'Ready', actions: [], access: 'supervised', moves: [], url: '', threadId: '',
  author: 'amy', assignees: [], labels: [], ignored: [], createdAt: number, updatedAt: number, ...extra,
})
const numbers = (list: Issue[]) => list.map((i) => i.number)

test('the default sort is the latest update, inside each section and in the sections order', () => {
  const issues = [issue(1, { updatedAt: 5, createdAt: 90 }), issue(2, { section: 'Ideas', updatedAt: 9 }), issue(3, { updatedAt: 8 })]
  assert.deepEqual(numbers(listIssues(issues, DEFAULT_ISSUE_PREFS, '')), [3, 1, 2])
  assert.deepEqual(numbers(listIssues(issues, { ...DEFAULT_ISSUE_PREFS, sort: 'created' }, '')), [1, 3, 2])
})

test('involvement needs the viewer and compares logins without case', () => {
  const issues = [issue(1, { assignees: ['Bo'] }), issue(2, { author: 'bo' }), issue(3)]
  assert.deepEqual(numbers(listIssues(issues, { ...DEFAULT_ISSUE_PREFS, involvement: 'assigned' }, 'bo')), [1])
  assert.deepEqual(numbers(listIssues(issues, { ...DEFAULT_ISSUE_PREFS, involvement: 'opened' }, 'bo')), [2])
  assert.deepEqual(numbers(listIssues(issues, { ...DEFAULT_ISSUE_PREFS, involvement: 'assigned' }, '')), [])
})

test('every label picked must be on the issue, and the other filters narrow too', () => {
  const issues = [issue(1, { labels: ['a', 'b'], type: 'feature' }), issue(2, { labels: ['a'] }), issue(3, { author: 'bo', labels: ['a', 'b'] })]
  assert.deepEqual(numbers(listIssues(issues, { ...DEFAULT_ISSUE_PREFS, labels: ['a', 'b'] }, '')).sort(), [1, 3])
  assert.deepEqual(numbers(listIssues(issues, { ...DEFAULT_ISSUE_PREFS, labels: ['a', 'b'], type: 'feature' }, '')), [1])
  assert.deepEqual(numbers(listIssues(issues, { ...DEFAULT_ISSUE_PREFS, author: 'bo' }, '')), [3])
  assert.deepEqual(numbers(listIssues(issues, { ...DEFAULT_ISSUE_PREFS, q: 'label:b -@bo' }, '')), [1])
})

test('the filter count is each filter set and a label each', () => {
  assert.equal(issueFilterCount(DEFAULT_ISSUE_PREFS), 0)
  assert.equal(issueFilterCount({ ...DEFAULT_ISSUE_PREFS, involvement: 'opened', type: 'bug', assignee: 'x', labels: ['a', 'b'] }), 5)
  assert.equal(issueFilterCount({ ...DEFAULT_ISSUE_PREFS, q: 'text', sort: 'created' }), 0)
})

test('the facets count what the issues have', () => {
  const f = issueFacets([issue(1, { labels: ['a'] }), issue(2, { labels: ['a', 'b'], type: 'chore' })])
  assert.deepEqual(f.types, [{ value: 'bug', count: 1 }, { value: 'chore', count: 1 }])
  assert.deepEqual(f.labels, [{ value: 'a', count: 2 }, { value: 'b', count: 1 }])
})
