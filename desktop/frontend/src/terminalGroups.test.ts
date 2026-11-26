import assert from 'node:assert/strict'
import { test } from 'node:test'
import { addGroup, closePane, keepRecent, MAX_PANES, reconcile, select, showsGroupHeaders, showsList, split, terminalLabel } from './terminalGroups.ts'
import type { ThreadTerms } from './terminalGroups.ts'

const two: ThreadTerms = {
  groups: [
    { id: 'g1', panes: ['a', 'b'], active: 'b' },
    { id: 'g2', panes: ['c'], active: 'c' },
  ],
  active: 'g1',
}

test('reconcile drops gone shells', () => {
  assert.deepEqual(reconcile(two, ['a', 'c']), {
    groups: [
      { id: 'g1', panes: ['a'], active: 'a' },
      { id: 'g2', panes: ['c'], active: 'c' },
    ],
    active: 'g1',
  })
  assert.deepEqual(reconcile(two, ['c']), { groups: [{ id: 'g2', panes: ['c'], active: 'c' }], active: 'g2' })
  assert.deepEqual(reconcile(two, []), { groups: [], active: '' })
})

test('a shell in no group gets its own group', () => {
  assert.deepEqual(reconcile(undefined, ['a', 'b']), {
    groups: [
      { id: 'g1', panes: ['a'], active: 'a' },
      { id: 'g2', panes: ['b'], active: 'b' },
    ],
    active: 'g1',
  })
  const after = reconcile(two, ['a', 'b', 'c', 'd'])
  assert.deepEqual(after.groups.at(-1), { id: 'g3', panes: ['d'], active: 'd' })
  assert.equal(after.active, 'g1')
})

test('a new terminal is a new group', () => {
  const t = addGroup(two, 'd')
  assert.deepEqual(t.groups.at(-1), { id: 'g3', panes: ['d'], active: 'd' })
  assert.equal(t.active, 'g3')
  assert.deepEqual(addGroup({ groups: [], active: '' }, 'a'), { groups: [{ id: 'g1', panes: ['a'], active: 'a' }], active: 'g1' })
})

test('split stops at four panes', () => {
  const t = split(two, 'd')
  assert.deepEqual(t.groups[0], { id: 'g1', panes: ['a', 'b', 'd'], active: 'd' })
  const full = split(t, 'e')
  assert.equal(full.groups[0].panes.length, MAX_PANES)
  assert.equal(split(full, 'f'), full)
  assert.deepEqual(split(select(two, 'a'), 'd').groups[0].panes, ['a', 'd', 'b'])
})

test('closing the last pane closes the group', () => {
  const t = select(two, 'c')
  assert.deepEqual(closePane(t, 'c'), { groups: [two.groups[0]], active: 'g1' })
  assert.deepEqual(closePane(two, 'b').groups[0], { id: 'g1', panes: ['a'], active: 'a' })
  assert.equal(closePane(two, 'c').active, 'g1')
})

test('the label is the process name', () => {
  assert.equal(terminalLabel('vim', 2), 'vim')
})

test('Terminal N with no process name', () => {
  assert.equal(terminalLabel('', 3), 'Terminal 3')
})

test('the list shows for two terminals', () => {
  assert.equal(showsList({ groups: [{ id: 'g1', panes: ['a'], active: 'a' }], active: 'g1' }), false)
  assert.equal(showsList({ groups: [{ id: 'g1', panes: ['a', 'b'], active: 'a' }], active: 'g1' }), true)
  assert.equal(showsList(two), true)
})

test('group headers show only with a split', () => {
  assert.equal(showsGroupHeaders(two), true)
  assert.equal(showsGroupHeaders(reconcile(undefined, ['a', 'b'])), false)
})

test('keepRecent keeps the ten newest', () => {
  let order: string[] = []
  for (let i = 1; i <= 12; i++) order = keepRecent(order, `t${i}`)
  assert.deepEqual(order, ['t12', 't11', 't10', 't9', 't8', 't7', 't6', 't5', 't4', 't3'])
  assert.deepEqual(keepRecent(order, 't5').slice(0, 2), ['t5', 't12'])
  assert.equal(keepRecent(order, 't5').length, 10)
})
