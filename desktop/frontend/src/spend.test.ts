import assert from 'node:assert/strict'
import { test } from 'node:test'
import { tokenCount } from './spend.ts'

test('a short token count keeps three figures at most', () => {
  assert.equal(tokenCount(412_000_000, true), '412M')
  assert.equal(tokenCount(4_400_000, true), '4.4M')
  assert.equal(tokenCount(57_340_000, true), '57.3M')
  assert.equal(tokenCount(12_345, true), '12.3k')
  assert.equal(tokenCount(999_700, true), '1M')
  assert.equal(tokenCount(850, true), '850')
})

test('a token count keeps its precision unless asked for short', () => {
  assert.equal(tokenCount(4_400_000), '4.40M')
  assert.equal(tokenCount(412_000_000), '412.0M')
})
