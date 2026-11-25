import assert from 'node:assert/strict'
import { test } from 'node:test'
import { nextView, shownView } from './detailsTabs.ts'

test('a hidden Agents tab shows PR', () => {
  assert.equal(shownView('agents', false), 'pr')
  assert.equal(shownView('agents', true), 'agents')
  assert.equal(shownView('browser', false), 'browser')
  assert.equal(shownView('servers', false), 'servers')
})

test('the tabs cycle among those shown, from the one shown', () => {
  assert.deepEqual([nextView('browser', 1, true, false), nextView('agents', 1, true, false), nextView('pr', -1, true, false)], ['agents', 'pr', 'agents'])
  assert.deepEqual([nextView('browser', 1, false, false), nextView('pr', -1, false, false)], ['pr', 'browser'], 'a hidden Agents tab is skipped')
  assert.deepEqual([nextView('agents', -1, false, false), nextView('agents', 1, false, false)], ['browser', 'evidence'], 'from a hidden Agents tab, as from PR it shows')
})

test('the Servers tab comes last while shown', () => {
  assert.deepEqual([nextView('agents', 1, true, true), nextView('servers', 1, true, true), nextView('pr', -1, true, true), nextView('servers', -1, true, true)], ['servers', 'pr', 'servers', 'agents'])
  assert.deepEqual([nextView('browser', 1, false, true), nextView('servers', -1, false, true)], ['servers', 'browser'], 'after Browser while the Agents tab is hidden')
  assert.deepEqual([nextView('browser', 1, true, false), nextView('agents', 1, true, false)], ['agents', 'pr'], 'a hidden Servers tab is skipped')
  assert.deepEqual([nextView('servers', 1, false, false), nextView('servers', -1, false, false)], ['pr', 'browser'], 'a chosen Servers tab shows with no server left')
})
