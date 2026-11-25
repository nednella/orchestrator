import assert from 'node:assert/strict'
import { test } from 'node:test'
import { isOpened, setOpened } from './chatOpen.ts'

test('an opened fold stays opened until it is closed', () => {
  setOpened('p/a', 'turn-1', true)
  assert.equal(isOpened('p/a', 'turn-1'), true)
  assert.equal(isOpened('p/b', 'turn-1'), false)
  setOpened('p/a', 'turn-1', false)
  assert.equal(isOpened('p/a', 'turn-1'), false)
})

test('the threads kept are bounded to 20', () => {
  for (let i = 0; i < 21; i++) setOpened(`t/${i}`, 'k', true)
  assert.equal(isOpened('t/0', 'k'), false)
  assert.equal(isOpened('t/20', 'k'), true)
})
