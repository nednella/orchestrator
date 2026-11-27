import assert from 'node:assert/strict'
import { test } from 'node:test'
import { moveOption } from './selectKeys.ts'

test('the arrows step through the options and stop at the ends', () => {
  assert.equal(moveOption('ArrowDown', 0, 3), 1)
  assert.equal(moveOption('ArrowDown', 2, 3), 2)
  assert.equal(moveOption('ArrowUp', 2, 3), 1)
  assert.equal(moveOption('ArrowUp', 0, 3), 0)
})

test('Home and End jump to the first and the last option', () => {
  assert.equal(moveOption('Home', 2, 3), 0)
  assert.equal(moveOption('End', 0, 3), 2)
})

test('another key moves nowhere', () => {
  assert.equal(moveOption('a', 1, 3), -1)
})
