import assert from 'node:assert/strict'
import { test } from 'node:test'
import { pickCursor, pickListKey } from './composerPick.ts'

test('pickListKey: Up and Down move within the list', () => {
  assert.deepEqual(pickListKey('ArrowDown', 3, -1), { move: 0 })
  assert.deepEqual(pickListKey('ArrowDown', 3, 2), { move: 2 })
  assert.deepEqual(pickListKey('ArrowUp', 3, 0), { move: 0 })
  assert.deepEqual(pickListKey('ArrowUp', 3, 2), { move: 1 })
})

test('pickListKey: Enter and Tab pick the cursor row, or the first', () => {
  assert.deepEqual(pickListKey('Enter', 3, 1), { pick: 1 })
  assert.deepEqual(pickListKey('Tab', 3, -1), { pick: 0 })
})

test('pickListKey: an empty list takes no key, and other keys pass through', () => {
  assert.equal(pickListKey('Enter', 0, -1), null)
  assert.equal(pickListKey('Escape', 3, 0), null)
  assert.equal(pickListKey('a', 3, 0), null)
})

test('pickCursor: the first row is highlighted until the cursor moves', () => {
  assert.equal(pickCursor(-1, 3), 0)
  assert.equal(pickCursor(2, 3), 2)
  assert.equal(pickCursor(5, 3), 2)
  assert.equal(pickCursor(-1, 0), -1)
})
