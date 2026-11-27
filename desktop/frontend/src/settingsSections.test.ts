import assert from 'node:assert/strict'
import { test } from 'node:test'
import { SETTINGS_NAV, sectionLabel } from './settingsSections.ts'

const ids = SETTINGS_NAV.flatMap((group) => group.sections.map((s) => s.id))

test('the nav lists each panel once, About last', () => {
  assert.equal(new Set(ids).size, ids.length)
  assert.equal(ids[ids.length - 1], 'about')
})

test('the panels come in the order of the groups', () => {
  assert.deepEqual(ids, ['appearance', 'threads', 'browser', 'worktrees', 'maintenance', 'about'])
})

test('a panel is named by its nav label', () => {
  assert.equal(sectionLabel('worktrees'), 'Worktrees')
  assert.equal(sectionLabel('about'), 'About')
})
