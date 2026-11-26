import assert from 'node:assert/strict'
import { test } from 'node:test'
import { closeTab, EMPTY_PANEL, openTab, panelFit, panelStateOf, panelWidth, stepTab, viewForKey } from './panelTabs.ts'
import type { PanelState } from './panelTabs.ts'

const state = (over: Partial<PanelState> = {}): PanelState => ({ ...EMPTY_PANEL, ...over })

test('opening a view adds its tab and makes it active', () => {
  assert.deepEqual(openTab(EMPTY_PANEL, 'agents'), { tabs: ['agents'], active: 'agents', expanded: false })
  assert.deepEqual(openTab(state({ tabs: ['agents'], active: 'agents', expanded: true }), 'issues'), { tabs: ['agents', 'issues'], active: 'issues', expanded: true })
})

test('opening an open view only selects it', () => {
  const st = state({ tabs: ['agents', 'servers', 'issues'], active: 'issues' })
  assert.deepEqual(openTab(st, 'agents'), { ...st, active: 'agents' })
  assert.deepEqual(openTab(state({ tabs: ['agents'], active: null }), 'agents'), { tabs: ['agents'], active: 'agents', expanded: false }, 'from the list')
})

test('closing the active tab picks its left neighbour', () => {
  const st = state({ tabs: ['agents', 'servers', 'issues'], active: 'servers', expanded: true })
  assert.deepEqual(closeTab(st, 'servers'), { tabs: ['agents', 'issues'], active: 'agents', expanded: true })
  assert.deepEqual(closeTab(st, 'agents'), { tabs: ['servers', 'issues'], active: 'servers', expanded: true }, 'a tab that is not active leaves the active one')
  assert.deepEqual(closeTab(state({ tabs: ['agents', 'servers'], active: 'agents' }), 'agents'), { tabs: ['servers'], active: 'servers', expanded: false }, 'the first tab has no left neighbour: the next one')
  assert.equal(closeTab(st, 'notes'), st, 'a view with no tab changes nothing')
})

test('closing the last tab shows the list and restores the size', () => {
  assert.deepEqual(closeTab(state({ tabs: ['issues'], active: 'issues', expanded: true }), 'issues'), EMPTY_PANEL)
  assert.deepEqual(closeTab(state({ tabs: ['issues'], active: null, expanded: true }), 'issues'), EMPTY_PANEL, 'from the list too')
})

test('letters open their views', () => {
  const keys: [string, string][] = [['a', 'agents'], ['S', 'servers'], ['p', 'pulls'], ['i', 'issues'], ['N', 'notes'], ['e', 'evidence'], ['b', 'browser']]
  for (const [key, view] of keys) assert.equal(viewForKey(key), view, key)
  for (const key of ['f', 'x', 'Enter', 'ArrowDown', '1', '']) assert.equal(viewForKey(key), null, `${key}: no view, Files comes with its view`)
})

test('the width keeps 360 px of chat', () => {
  assert.deepEqual(panelWidth(1512, 1512), { min: 360, max: 1058, def: 540 }, '70% of the window, in whole px')
  assert.deepEqual(panelWidth(1512, 1100), { min: 360, max: 740, def: 540 }, 'the row less 360 px')
  assert.equal(panelFit(1512, 1100, null).width, 540, 'the default')
  assert.equal(panelFit(1512, 1100, 900).width, 740, 'a saved width that would leave the chat under 360 px')
  assert.equal(panelFit(1512, 1100, 200).width, 360, 'never under 360 px')
  assert.equal(panelFit(1000, 660, null).width, 360, 'with no room for both, the panel keeps its least')
})

test('a window of 980 px or less shows the panel as a sheet', () => {
  assert.deepEqual(panelFit(981, 981, null), { docked: true, width: 540 })
  assert.deepEqual(panelFit(980, 980, 600), { docked: false, width: 411 }, '42% of the window')
  assert.deepEqual(panelFit(1200 * 0.8, 700, null), { docked: false, width: 403 })
  assert.deepEqual(panelFit(1100, 1100, null).docked, true)
  assert.deepEqual(panelFit(700, 700, null), { docked: false, width: 384 }, '88% of the window, at most 24rem')
  assert.deepEqual(panelFit(760, 760, null), { docked: false, width: 384 })
  assert.deepEqual(panelFit(400, 400, null), { docked: false, width: 352 })
  assert.deepEqual(panelFit(800, 800, null), { docked: false, width: 336 })
  assert.deepEqual(panelFit(770, 770, null), { docked: false, width: 323 })
  assert.deepEqual(panelFit(761, 761, null), { docked: false, width: 320 }, 'at least 320 px')
})

test('the tab keys step through the open tabs, round the ends', () => {
  const st = state({ tabs: ['agents', 'servers', 'issues'], active: 'servers' })
  assert.equal(stepTab(st, 1).active, 'issues')
  assert.equal(stepTab(stepTab(st, 1), 1).active, 'agents')
  assert.equal(stepTab(st, -1).active, 'agents')
  assert.equal(stepTab(state({ tabs: ['agents', 'servers'], active: null }), 1).active, 'agents', 'from the list, the first')
  assert.equal(stepTab(state({ tabs: ['agents', 'servers'], active: null }), -1).active, 'servers', 'from the list back, the last')
  assert.equal(stepTab(EMPTY_PANEL, 1), EMPTY_PANEL)
})

test('a saved panel keeps only the views it knows', () => {
  assert.deepEqual(panelStateOf(undefined), EMPTY_PANEL)
  assert.deepEqual(panelStateOf({ tabs: ['agents', 'details', 'agents', 'files'], active: 'details', expanded: true }), { tabs: ['agents'], active: null, expanded: true })
  assert.deepEqual(panelStateOf({ tabs: ['issues'], active: 'issues', expanded: false }), { tabs: ['issues'], active: 'issues', expanded: false })
  assert.deepEqual(panelStateOf({ tabs: 'issues', active: 7, expanded: 'yes' }), EMPTY_PANEL)
})
