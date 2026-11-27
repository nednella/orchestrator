import assert from 'node:assert/strict'
import { test } from 'node:test'
import { forgettable } from './deleting.ts'

test('a thread whose delete runs is not forgotten when its row leaves the list', () => {
  const deleting = new Map([['shop/a', ['shop/a', 'shop/b']]])
  assert.deepEqual(forgettable(['shop/a', 'shop/b', 'shop/c'], deleting), ['shop/c'])
})

test('every thread is forgotten once no delete holds it, as after a refusal or the end of a delete', () => {
  assert.deepEqual(forgettable(['shop/a', 'shop/b'], new Map()), ['shop/a', 'shop/b'])
})
