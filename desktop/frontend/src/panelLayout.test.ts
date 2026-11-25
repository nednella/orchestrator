import assert from 'node:assert/strict'
import { test } from 'node:test'
import { fit } from './panelLayout.ts'

const ROOT = 16

test('the sidebar keeps the width it was set to at every window width', () => {
  for (const window of [640, 700, 899, 900, 1100, 1249, 1250, 1512, 2560]) {
    assert.equal(fit(window, { sidebarOpen: true, rightOpen: false, sidebar: 21, right: null }, ROOT).widths.sidebar, 21, `${window}px`)
    assert.equal(fit(window, { sidebarOpen: true, rightOpen: true, sidebar: 21, right: 20 }, ROOT).widths.sidebar, 21, `${window}px with the right panel`)
  }
})

test('the right panel shrinks for the thread, then closes, and the sidebar does not give way', () => {
  const prefs = { sidebarOpen: true, rightOpen: true, sidebar: 18, right: 20 }
  assert.deepEqual(fit(1512, prefs, ROOT), { widths: { sidebar: 18, right: 20 }, rightForcedClosed: false })
  const shrunk = fit(900, prefs, ROOT)
  assert.equal(shrunk.rightForcedClosed, false)
  assert.equal(Math.round(shrunk.widths.right * ROOT), 900 - 21 * ROOT - 3 - 280)
  assert.deepEqual(fit(700, prefs, ROOT), { widths: { sidebar: 18, right: 0 }, rightForcedClosed: true })
})

test('a closed right panel reports the width it opens at', () => {
  assert.equal(fit(600, { sidebarOpen: false, rightOpen: false, sidebar: 18, right: 15 }, ROOT).widths.right, 15)
  assert.equal(fit(600, { sidebarOpen: false, rightOpen: false, sidebar: 18, right: null }, ROOT).widths.right, 9.5)
  assert.equal(fit(1512, { sidebarOpen: false, rightOpen: false, sidebar: 18, right: null }, ROOT).widths.right, 22)
  assert.equal(fit(1200, { sidebarOpen: false, rightOpen: false, sidebar: 18, right: null }, ROOT).widths.right, (0.24 * 1200) / ROOT)
})
