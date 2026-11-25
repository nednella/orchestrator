import assert from 'node:assert/strict'
import { test } from 'node:test'
import { fit, modeFor, nextOverlay, OVERLAY_BELOW } from './panelLayout.ts'

const ROOT = 16

const prefs = (sidebar: number, right: number | null = null, sidebarOpen = true) => ({ sidebarOpen, sidebar, right })

test('the sidebar keeps the width it was set to at every window width', () => {
  for (const windowPx of [600, 640, 700, 899, 900, 1100, 1249, 1250, 1512, 2560]) {
    assert.equal(fit(windowPx, prefs(21), ROOT).widths.sidebar, 21, `${windowPx}px`)
    assert.equal(fit(windowPx, prefs(21, 20), ROOT).widths.sidebar, 21, `${windowPx}px with the right panel`)
  }
})

test('the sidebar docks only where the thread keeps its room beside it', () => {
  assert.equal(fit(1512, prefs(18), ROOT).docked.sidebar, true)
  assert.equal(fit(819, prefs(18), ROOT).docked.sidebar, true)
  assert.equal(fit(818, prefs(18), ROOT).docked.sidebar, false)
  assert.equal(fit(700, prefs(18), ROOT).docked.sidebar, false)
  assert.equal(fit(640, prefs(32), ROOT).docked.sidebar, false)
  assert.equal(fit(1043, prefs(32), ROOT).docked.sidebar, true)
  assert.equal(fit(1042, prefs(32), ROOT).docked.sidebar, false)
})

test('below the breakpoint nothing docks, whatever the room', () => {
  assert.deepEqual(fit(OVERLAY_BELOW - 1, prefs(13, 9.5, false), ROOT).docked, { sidebar: false, right: false })
})

test('the right panel docks only where it fits at its width beside the thread', () => {
  assert.deepEqual(fit(1512, prefs(18), ROOT), { widths: { sidebar: 18, right: 22 }, docked: { sidebar: true, right: true } })
  assert.deepEqual(fit(900, prefs(18), ROOT).docked, { sidebar: true, right: false })
  assert.deepEqual(fit(900, prefs(18, null, false), ROOT), { widths: { sidebar: 18, right: 22 }, docked: { sidebar: true, right: true } })
  assert.equal(fit(1512, prefs(18, 32), ROOT).docked.right, true)
  assert.equal(fit(1200, prefs(18, 32), ROOT).docked.right, false)
})

test('the right panel has one width, docked or over the thread', () => {
  assert.deepEqual(fit(1512, prefs(18), ROOT), { widths: { sidebar: 18, right: 22 }, docked: { sidebar: true, right: true } })
  assert.deepEqual(fit(900, prefs(18), ROOT), { widths: { sidebar: 18, right: 22 }, docked: { sidebar: true, right: false } })
  assert.deepEqual(fit(700, prefs(18), ROOT), { widths: { sidebar: 18, right: 22 }, docked: { sidebar: false, right: false } })
  assert.equal(fit(1512, prefs(18, 15), ROOT).widths.right, 15)
  assert.equal(fit(700, prefs(18, 15), ROOT).widths.right, 15)
})

test('a panel over the thread opens at its full width, capped by the room under the top bar', () => {
  assert.equal(fit(900, prefs(18), ROOT).widths.right, 22)
  assert.equal(fit(600, prefs(18), ROOT).widths.right, 22)
  assert.equal(fit(600, prefs(18, 25), ROOT).widths.right, 25)
  assert.equal(fit(300, prefs(18), ROOT).widths.right, 300 / ROOT)
  assert.equal(fit(700, prefs(18), ROOT).widths.sidebar, 18)
  assert.equal(fit(300, prefs(18), ROOT).widths.sidebar, 300 / ROOT - 3)
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
