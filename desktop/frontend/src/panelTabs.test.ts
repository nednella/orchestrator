import assert from 'node:assert/strict'
import { test } from 'node:test'
import { sidebarBesidePx, sidebarFit } from './panelLayout.ts'
import { closeTab, EMPTY_PANEL, openTab, panelFit, panelOnFocus, panelStateOf, panelWidth, sideChatOf, sideChatTab, stepTab, targetForKey } from './panelTabs.ts'
import type { PanelState } from './panelTabs.ts'

const state = (over: Partial<PanelState> = {}): PanelState => ({ ...EMPTY_PANEL, ...over })

test('opening a view adds its tab and makes it active', () => {
  assert.deepEqual(openTab(EMPTY_PANEL, 'notes'), { tabs: ['notes'], active: 'notes', expanded: false })
  assert.deepEqual(openTab(state({ tabs: ['notes'], active: 'notes', expanded: true }), 'issues'), { tabs: ['notes', 'issues'], active: 'issues', expanded: true })
})

test('opening an open view only selects it', () => {
  const st = state({ tabs: ['notes', 'files', 'issues'], active: 'issues' })
  assert.deepEqual(openTab(st, 'notes'), { ...st, active: 'notes' })
  assert.deepEqual(openTab(state({ tabs: ['notes'], active: null }), 'notes'), { tabs: ['notes'], active: 'notes', expanded: false }, 'from the list')
})

test('closing the active tab picks its left neighbour', () => {
  const st = state({ tabs: ['notes', 'files', 'issues'], active: 'files', expanded: true })
  assert.deepEqual(closeTab(st, 'files'), { tabs: ['notes', 'issues'], active: 'notes', expanded: true })
  assert.deepEqual(closeTab(st, 'notes'), { tabs: ['files', 'issues'], active: 'files', expanded: true }, 'a tab that is not active leaves the active one')
  assert.deepEqual(closeTab(state({ tabs: ['notes', 'files'], active: 'notes' }), 'notes'), { tabs: ['files'], active: 'files', expanded: false }, 'the first tab has no left neighbour: the next one')
  assert.equal(closeTab(st, 'browser'), st, 'a view with no tab changes nothing')
})

test('closing the last tab shows the list and restores the size', () => {
  assert.deepEqual(closeTab(state({ tabs: ['issues'], active: 'issues', expanded: true }), 'issues'), EMPTY_PANEL)
  assert.deepEqual(closeTab(state({ tabs: ['issues'], active: null, expanded: true }), 'issues'), EMPTY_PANEL, 'from the list too')
})

test('a side chat opens as a tab of its own, beside the views', () => {
  const tab = sideChatTab('main/abcdefgh')
  const st = openTab(state({ tabs: ['notes'], active: 'notes' }), tab)
  assert.deepEqual(st, { tabs: ['notes', 'side:main/abcdefgh'], active: tab, expanded: false })
  assert.equal(sideChatOf(st.active), 'main/abcdefgh')
  assert.equal(sideChatOf('notes'), null, 'a view is no side chat')
  assert.deepEqual(closeTab(st, tab), { tabs: ['notes'], active: 'notes', expanded: false })
  assert.deepEqual(panelStateOf(st), st, 'a saved side chat tab is kept')
  assert.deepEqual(panelStateOf({ tabs: ['side:', 'notes'], active: 'side:' }), { tabs: ['notes'], active: null, expanded: false }, 'one with no id is dropped')
})

test('letters open their views, and S starts a side chat', () => {
  const keys: [string, string][] = [['n', 'notes'], ['F', 'files'], ['i', 'issues'], ['p', 'pulls'], ['B', 'browser'], ['m', 'media'], ['w', 'news'], ['t', 'terminal'], ['s', 'sideChat'], ['S', 'sideChat']]
  for (const [key, view] of keys) assert.equal(targetForKey(key), view, key)
  for (const key of ['a', 'e', 'Enter', 'ArrowDown', '1', '']) assert.equal(targetForKey(key), null, `${key}: no view`)
})

test('the width keeps 360 px of chat', () => {
  assert.deepEqual(panelWidth(1512, 1512), { min: 360, max: 1058, def: 540 }, '70% of the window, in whole px')
  assert.deepEqual(panelWidth(1512, 1100), { min: 360, max: 740, def: 540 }, 'the row less 360 px')
  assert.equal(panelFit(1512, 1100, null).width, 540, 'the default')
  assert.equal(panelFit(1512, 1100, 900).width, 740, 'a saved width that would leave the chat under 360 px')
  assert.equal(panelFit(1512, 1100, 200).width, 360, 'never under 360 px')
  assert.deepEqual(panelFit(1000, 720, null), { docked: true, width: 360 }, 'room for both at their least')
  assert.deepEqual(panelFit(1000, 719, null), { docked: false, width: 420 }, 'no room for both: a sheet')
})

test('the open sidebar sends the panel over the chat where the chat would get under 360 px', () => {
  const row = (windowPx: number, sidebar: number) => windowPx - sidebarBesidePx(sidebarFit(windowPx, sidebar, 16), true, 16) - 1
  assert.equal(panelFit(981, row(981, 18), null).docked, false, '981 px with an 18rem sidebar')
  assert.equal(panelFit(1057, row(1057, 18), null).docked, false, '1057 px with an 18rem sidebar')
  assert.deepEqual(panelFit(1058, row(1058, 18), null), { docked: true, width: 360 }, 'from 1058 px both fit')
  assert.equal(panelFit(1281, row(1281, 32), null).docked, false, '1281 px with a 32rem sidebar')
  assert.deepEqual(panelFit(1282, row(1282, 32), null), { docked: true, width: 360 })
  for (const windowPx of [981, 1000, 1057, 1058, 1100, 1281, 1282, 1512]) {
    for (const sidebar of [13, 18, 32]) {
      const r = row(windowPx, sidebar)
      const fit = panelFit(windowPx, r, null)
      if (fit.docked) assert.equal(r - fit.width >= 360, true, `${windowPx} px, ${sidebar}rem: the chat keeps ${r - fit.width} px`)
    }
  }
})

test('a focus in the chat column, or News opening there, gives it its room back', () => {
  const expanded = state({ tabs: ['issues'], active: 'issues', expanded: true })
  for (const target of ['thread', 'composer'] as const) assert.deepEqual(panelOnFocus(expanded, target), { ...expanded, expanded: false }, target)
  for (const target of ['right', 'sidebar', 'thread-search', 'queue-filter'] as const) assert.equal(panelOnFocus(expanded, target), expanded, target)
  const restored = state({ tabs: ['issues'], active: 'issues' })
  assert.equal(panelOnFocus(restored, 'thread'), restored, 'a panel at its own size stays as it is')
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
  const st = state({ tabs: ['notes', 'files', 'issues'], active: 'files' })
  assert.equal(stepTab(st, 1).active, 'issues')
  assert.equal(stepTab(stepTab(st, 1), 1).active, 'notes')
  assert.equal(stepTab(st, -1).active, 'notes')
  assert.equal(stepTab(state({ tabs: ['notes', 'files'], active: null }), 1).active, 'notes', 'from the list, the first')
  assert.equal(stepTab(state({ tabs: ['notes', 'files'], active: null }), -1).active, 'files', 'from the list back, the last')
  assert.equal(stepTab(EMPTY_PANEL, 1), EMPTY_PANEL)
})

test('a saved panel keeps only the views it knows', () => {
  assert.deepEqual(panelStateOf(undefined), EMPTY_PANEL)
  assert.deepEqual(panelStateOf({ tabs: ['notes', 'details', 'notes', 'diff', 'files'], active: 'details', expanded: true }), { tabs: ['notes', 'files'], active: null, expanded: true })
  assert.deepEqual(panelStateOf({ tabs: ['agents', 'servers', 'issues'], active: 'agents', expanded: false }), { tabs: ['issues'], active: null, expanded: false }, 'Agents and Servers are gone')
  assert.deepEqual(panelStateOf({ tabs: ['evidence'], active: 'evidence', expanded: false }), { tabs: ['media'], active: 'media', expanded: false }, 'Evidence was saved as Media')
  assert.deepEqual(panelStateOf({ tabs: ['issues'], active: 'issues', expanded: false }), { tabs: ['issues'], active: 'issues', expanded: false })
  assert.deepEqual(panelStateOf({ tabs: 'issues', active: 7, expanded: 'yes' }), EMPTY_PANEL)
})

test('the Terminal view is shown with no tab of its own, since its tabs are the project terminals', () => {
  const st = state({ tabs: ['notes'], active: 'notes', expanded: true })
  assert.deepEqual(openTab(st, 'terminal'), { tabs: ['notes'], active: 'terminal', expanded: true })
  assert.deepEqual(panelStateOf({ tabs: ['notes', 'terminal'], active: 'terminal', expanded: false }), { tabs: ['notes'], active: 'terminal', expanded: false })
})

test('closing the last view tab keeps the Terminal view shown', () => {
  assert.deepEqual(closeTab(state({ tabs: ['notes'], active: 'terminal' }), 'notes'), { tabs: [], active: 'terminal', expanded: false })
})

test('stepping from the terminal tabs goes to the first or the last view', () => {
  const st = state({ tabs: ['notes', 'files', 'issues'], active: 'terminal' })
  assert.equal(stepTab(st, 1).active, 'notes')
  assert.equal(stepTab(st, -1).active, 'issues')
})
