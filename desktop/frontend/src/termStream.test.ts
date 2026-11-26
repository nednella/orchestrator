import assert from 'node:assert/strict'
import { test } from 'node:test'
import { keepsStream, servedOverHTTP } from './termStream.ts'

test('in the window a kept terminal keeps its stream', () => {
  assert.equal(keepsStream(false, true), true)
  assert.equal(keepsStream(false, false), true)
})

test('in a browser only a terminal on screen keeps its stream', () => {
  assert.equal(keepsStream(true, true), true)
  assert.equal(keepsStream(true, false), false)
})

test('a page over http is a browser', () => {
  assert.equal(servedOverHTTP('http:'), true)
  assert.equal(servedOverHTTP('https:'), true)
  assert.equal(servedOverHTTP('wails:'), false)
})
