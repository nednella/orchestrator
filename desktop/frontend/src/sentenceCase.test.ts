import assert from 'node:assert/strict'
import { test } from 'node:test'
import { sentenceCase } from './sentenceCase.ts'

test('sentenceCase lowers every word after the first', () => {
  assert.equal(sentenceCase('Performance Improvements'), 'Performance improvements')
  assert.equal(sentenceCase('Bug Fixes'), 'Bug fixes')
  assert.equal(sentenceCase('features'), 'Features')
  assert.equal(sentenceCase(''), '')
})
