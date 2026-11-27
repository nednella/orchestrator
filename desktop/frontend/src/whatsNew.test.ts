import assert from 'node:assert/strict'
import { test } from 'node:test'
import { overlayAtStart } from './whatsNew.ts'
import type { Release } from './types.ts'

const notes: Release[] = [{ version: '0.2.0', date: '2026-11-27', sections: [] }]

test('the notes of a new version open the dialog', () => {
  assert.equal(overlayAtStart(null, notes), 'changelog')
})

test('no notes, as for a dev build or an unreadable release, open nothing', () => {
  assert.equal(overlayAtStart(null, []), null)
})

test('an overlay the owner already opened stays', () => {
  assert.equal(overlayAtStart('settings', notes), 'settings')
  assert.equal(overlayAtStart('settings', []), 'settings')
})
