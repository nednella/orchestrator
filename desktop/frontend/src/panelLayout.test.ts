import assert from 'node:assert/strict'
import { test } from 'node:test'
import { closesPanel, modeFor, nextOverlay, OVERLAY_BELOW, RAIL_REM, SIDEBAR_MIN_REM, sidebarFit, sidebarMax } from './panelLayout.ts'
import { panelFit } from './panelTabs.ts'

const ROOT = 16

test('the sidebar and the rail are at least 300 px wide at the default text size', () => {
  assert.equal((RAIL_REM + SIDEBAR_MIN_REM) * ROOT, 300)
})

test('the sidebar keeps the width it was set to at every window width', () => {
  for (const windowPx of [600, 640, 700, 899, 900, 1100, 1249, 1250, 1512, 2560]) assert.equal(sidebarFit(windowPx, 21, ROOT).width, 21, `${windowPx}px`)
})

test('the sidebar docks only where the thread keeps its room beside it', () => {
  assert.equal(sidebarFit(1512, 18, ROOT).docked, true)
  assert.equal(sidebarFit(819, 18, ROOT).docked, true)
  assert.equal(sidebarFit(818, 18, ROOT).docked, false)
  assert.equal(sidebarFit(700, 18, ROOT).docked, false)
  assert.equal(sidebarFit(640, 32, ROOT).docked, false)
  assert.equal(sidebarFit(1043, 32, ROOT).docked, true)
  assert.equal(sidebarFit(1042, 32, ROOT).docked, false)
})

test('below the breakpoint the sidebar never docks, whatever the room', () => {
  assert.equal(sidebarFit(OVERLAY_BELOW - 1, 13, ROOT).docked, false)
})

test('a sidebar over the thread is never wider than the window', () => {
  assert.equal(sidebarFit(700, 18, ROOT).width, 18)
  assert.equal(sidebarFit(300, 18, ROOT).width, 300 / ROOT - 3)
})

test('a drag of the docked sidebar stops where it would leave the side of the thread', () => {
  for (const root of [16, 17.6, 19.2]) {
    for (const windowPx of [820, 1000, 1512, 2560]) {
      const max = sidebarMax(windowPx, 0, root)
      assert.equal(sidebarFit(windowPx, max, root).docked, true, `${windowPx}px at ${root}`)
      assert.equal(sidebarFit(windowPx, max + 0.02, root).docked, false, `${windowPx}px at ${root}, past the limit`)
    }
  }
  assert.equal(sidebarMax(1512, 0, ROOT), 61.31)
})

test('a drag of the docked sidebar leaves the open right panel and the chat their room', () => {
  const max = sidebarMax(1512, 301, ROOT)
  assert.equal(max, 42.5)
  const row = 1512 - (3 + max) * ROOT - 3
  const { width } = panelFit(1512, row, 540)
  assert.equal(width >= 300 && row - width >= 360, true, `the panel at ${width}px leaves the chat ${row - width}px`)
})

test('closing an overlay closes its panel, opening a thread only puts it away', () => {
  assert.equal(closesPanel({ close: 'right' }), true)
  assert.equal(closesPanel('escape'), true)
  assert.equal(closesPanel('thread-focused'), true)
  assert.equal(closesPanel('thread-opened'), false)
  assert.equal(closesPanel({ open: 'right' }), false)
})

test('the window width picks the layout band', () => {
  assert.equal(modeFor(OVERLAY_BELOW - 1), 'narrow')
  assert.equal(modeFor(600), 'narrow')
  assert.equal(modeFor(OVERLAY_BELOW), 'compact')
  assert.equal(modeFor(900), 'medium')
  assert.equal(modeFor(1512), 'wide')
})

test('a trigger opens its overlay and closes it again', () => {
  assert.equal(nextOverlay(null, { open: 'sidebar' }), 'sidebar')
  assert.equal(nextOverlay('sidebar', { close: 'sidebar' }), null)
  assert.equal(nextOverlay(null, { open: 'right' }), 'right')
  assert.equal(nextOverlay('right', { close: 'right' }), null)
})

test('only one overlay is open at a time', () => {
  assert.equal(nextOverlay('sidebar', { open: 'right' }), 'right')
  assert.equal(nextOverlay('right', { open: 'sidebar' }), 'sidebar')
  assert.equal(nextOverlay('right', { close: 'sidebar' }), 'right')
  assert.equal(nextOverlay('sidebar', { close: 'right' }), 'sidebar')
})

test('Esc, the thread beside it and opening a thread close either overlay', () => {
  for (const open of ['sidebar', 'right'] as const) {
    assert.equal(nextOverlay(open, 'escape'), null, `${open} on Esc`)
    assert.equal(nextOverlay(open, 'thread-focused'), null, `${open} on a click on the thread`)
    assert.equal(nextOverlay(open, 'thread-opened'), null, `${open} when a thread opens`)
  }
})
