import assert from 'node:assert/strict'
import { test } from 'node:test'
import { branchFromTitle, freeBranch } from './branch.ts'

test('a title becomes a lower case branch with single dashes', () => {
  assert.equal(branchFromTitle('Fix the  Cart: rounding!'), 'fix-the-cart-rounding')
  assert.equal(branchFromTitle('  --Hello--  '), 'hello')
})

test('nothing left is thread', () => {
  assert.equal(branchFromTitle(''), 'thread')
  assert.equal(branchFromTitle('!!! ???'), 'thread')
})

test('a long title is cut at a dash within 40 characters', () => {
  const name = branchFromTitle('Why does checkout call the cart API twice when a user opens it')
  assert.equal(name, 'why-does-checkout-call-the-cart-api')
  assert.equal(name.length <= 40, true)
  assert.equal(branchFromTitle('a'.repeat(60)), 'a'.repeat(40))
})

test('a taken name gets -2, -3', () => {
  const taken = new Set(['fix', 'fix-2'])
  assert.equal(freeBranch('fix', (b) => taken.has(b)), 'fix-3')
  assert.equal(freeBranch('new', (b) => taken.has(b)), 'new')
})
