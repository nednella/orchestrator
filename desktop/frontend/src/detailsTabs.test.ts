import assert from 'node:assert/strict'
import { test } from 'node:test'
import { nextView, shownView } from './detailsTabs.ts'

test('a hidden Agents tab shows PR', () => {
  assert.equal(shownView('agents', false), 'pr')
  assert.equal(shownView('agents', true), 'agents')
  assert.equal(shownView('browser', false), 'browser')
})

test('the tabs cycle among those shown, from the one shown', () => {
  assert.deepEqual([nextView('browser', 1, true), nextView('agents', 1, true), nextView('pr', -1, true)], ['agents', 'pr', 'agents'])
  assert.deepEqual([nextView('browser', 1, false), nextView('pr', -1, false)], ['pr', 'browser'], 'a hidden Agents tab is skipped')
  assert.deepEqual([nextView('agents', -1, false), nextView('agents', 1, false)], ['browser', 'evidence'], 'from a hidden Agents tab, as from PR it shows')
})
