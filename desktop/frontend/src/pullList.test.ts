import assert from 'node:assert/strict'
import { test } from 'node:test'
import { DEFAULT_PULL_PREFS, listPulls, pullFilterCount } from './pullList.ts'
import type { RepoPull } from './types.ts'

const pull = (number: number, extra: Partial<RepoPull> = {}): RepoPull => ({
  number, title: `PR ${number}`, draft: false, branch: `b${number}`, url: '', author: 'amy', createdAt: number, updatedAt: 100 - number, labels: [], asked: [], thread: '', ...extra,
})
const numbers = (list: RepoPull[]) => list.map((p) => p.number)

test('the sorts order by update, newest and oldest', () => {
  const pulls = [pull(1), pull(2), pull(3)]
  assert.deepEqual(numbers(listPulls(pulls, DEFAULT_PULL_PREFS, '')), [1, 2, 3])
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, sort: 'created' }, '')), [3, 2, 1])
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, sort: 'oldest' }, '')), [1, 2, 3])
})

test('reviewing is a review asked of the viewer, authored is their own', () => {
  const pulls = [pull(1, { asked: ['Bo'] }), pull(2, { author: 'bo' }), pull(3)]
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, involvement: 'reviewing' }, 'bo')), [1])
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, involvement: 'authored' }, 'bo')), [2])
})

test('drafts, author and labels narrow the list', () => {
  const pulls = [pull(1, { draft: true, labels: ['x'] }), pull(2, { author: 'bo', labels: ['x', 'y'] }), pull(3)]
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, draft: 'only' }, '')), [1])
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, draft: 'hide' }, '')).sort(), [2, 3])
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, author: 'bo' }, '')), [2])
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, labels: ['x', 'y'] }, '')), [2])
})

test('the search text matches number, title, branch and author, with label: and @ tokens', () => {
  const pulls = [pull(1, { title: 'Fix the cart', labels: ['bug'] }), pull(2, { author: 'bo', branch: 'bo/cdn' })]
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, q: 'cart' }, '')), [1])
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, q: 'cdn' }, '')), [2])
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, q: '#2' }, '')), [2])
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, q: 'label:bug' }, '')), [1])
  assert.deepEqual(numbers(listPulls(pulls, { ...DEFAULT_PULL_PREFS, q: '@bo' }, '')), [2])
})

test('the filter count is each filter set and a label each', () => {
  assert.equal(pullFilterCount(DEFAULT_PULL_PREFS), 0)
  assert.equal(pullFilterCount({ ...DEFAULT_PULL_PREFS, involvement: 'authored', draft: 'hide', author: 'x', labels: ['a', 'b'] }), 5)
})
