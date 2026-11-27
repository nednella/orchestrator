import assert from 'node:assert/strict'
import { test } from 'node:test'
import { pullRows, pullsShown } from './pullRows.ts'
import type { RepoPull, Thread } from './types.ts'

const pull = (number: number, thread: string): RepoPull => ({ number, title: `PR ${number}`, draft: false, branch: `b${number}`, url: '', author: 'amy', createdAt: number, updatedAt: number, labels: [], asked: [], thread })
const thread = { id: 'p/aaaaaaaa' } as Thread

test("the thread's PR comes first", () => {
  const pulls = [pull(3, ''), pull(2, 'p/aaaaaaaa'), pull(1, 'p/bbbbbbbb')]
  const { mine, rest } = pullRows(pulls, thread)
  assert.equal(mine?.number, 2)
  assert.deepEqual(rest.map((p) => p.number), [3, 1])
})

test('no mine without a thread', () => {
  const pulls = [pull(2, 'p/aaaaaaaa'), pull(1, '')]
  const { mine, rest } = pullRows(pulls, null)
  assert.equal(mine, null)
  assert.deepEqual(rest.map((p) => p.number), [2, 1])
})

test("a read of another project is not the project's own", () => {
  const known = new Map([['b', [pull(7, '')]]])
  const read = { project: 'a', pulls: [pull(1, '')], error: 'x' }
  assert.equal(pullsShown(read, 'a', known).pulls?.[0].number, 1)
  assert.equal(pullsShown(read, 'a', known).error, 'x')
  assert.equal(pullsShown(read, 'b', known).pulls?.[0].number, 7)
  assert.equal(pullsShown(read, 'c', known).pulls, null)
})
