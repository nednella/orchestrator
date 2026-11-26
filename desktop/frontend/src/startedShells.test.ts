import assert from 'node:assert/strict'
import { test } from 'node:test'
import { START_GRACE_MS, stillStarted } from './startedShells.ts'
import type { Started } from './startedShells.ts'

const started: Started = new Map([['p/shell-2', { project: 'p', at: 1000 }]])

test('a started shell stays while no announce lists it and the grace has not passed', () => {
  assert.deepEqual([...stillStarted(started, 'p', new Set(['p/shell']), 1000 + START_GRACE_MS - 1).keys()], ['p/shell-2'])
})

test('an announce that lists it ends the wait', () => {
  assert.equal(stillStarted(started, 'p', new Set(['p/shell-2']), 1001).size, 0)
})

test('a shell no announce lists is dropped once the grace has passed', () => {
  assert.equal(stillStarted(started, 'p', new Set(), 1000 + START_GRACE_MS).size, 0)
})

test("another project's started shells are dropped", () => {
  assert.equal(stillStarted(started, 'q', new Set(), 1001).size, 0)
})
