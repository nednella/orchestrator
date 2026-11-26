import assert from 'node:assert/strict'
import { test } from 'node:test'
import { clickCloses } from './threadClick.ts'

test('a single click on the open thread closes it', () => {
  assert.equal(clickCloses(true, 1), true)
  assert.equal(clickCloses(true, 0), true)
})

test('the second click of a double-click does not close it', () => {
  assert.equal(clickCloses(true, 2), false)
})

test('a click on a thread that is not open never closes', () => {
  assert.equal(clickCloses(false, 1), false)
})
