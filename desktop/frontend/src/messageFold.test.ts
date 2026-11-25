import assert from 'node:assert/strict'
import { test } from 'node:test'
import { folds } from './messageFold.ts'

test('a message folds only once it is taller than the folded height and its fade', () => {
  assert.equal(folds(120, 16), false)
  assert.equal(folds(204, 16), false)
  assert.equal(folds(205, 16), true)
})

test('the fold scales with the text', () => {
  assert.equal(folds(240, 20), false)
  assert.equal(folds(256, 20), true)
})
