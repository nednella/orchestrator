import assert from 'node:assert/strict'
import { test } from 'node:test'
import { watchAgain } from './chatLive.ts'

test('watches again after a wake', () => {
  assert.equal(watchAgain('settled', 'idle'), true)
  assert.equal(watchAgain('settled', 'working'), true)
})

test('not on other changes', () => {
  assert.equal(watchAgain(undefined, 'idle'), false, 'the first state the chat sees')
  assert.equal(watchAgain('idle', 'working'), false)
  assert.equal(watchAgain('working', 'settled'), false)
  assert.equal(watchAgain('settled', 'settled'), false)
})
