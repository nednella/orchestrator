import assert from 'node:assert/strict'
import { test } from 'node:test'
import { footerMode } from './composerLayout.ts'

test('compact under 620 px', () => {
  assert.equal(footerMode(560), 'compact')
  assert.equal(footerMode(619.5), 'compact')
  assert.equal(footerMode(620), 'wide')
  assert.equal(footerMode(736), 'wide')
})
