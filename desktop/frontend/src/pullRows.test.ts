import assert from 'node:assert/strict'
import { test } from 'node:test'
import { pullRows } from './pullRows.ts'
import type { RepoPull, Session } from './types.ts'

const pull = (number: number, thread: string): RepoPull => ({ number, title: `PR ${number}`, draft: false, branch: `b${number}`, url: '', author: 'amy', updatedAt: number, thread })
const thread = { id: 'p/aaaaaaaa' } as Session

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
