import assert from 'node:assert/strict'
import { test } from 'node:test'
import { addGroup, closePane, folderName, MAX_PANES, prune, reconcile, select, selectGroup, showsList, split, terminalLabel, terminalNumber, withShells } from './terminalGroups.ts'
import type { ProjectTerms } from './terminalGroups.ts'

const two: ProjectTerms = {
  groups: [
    { id: 'g1', panes: ['a', 'b'], active: 'b', dir: 'row' },
    { id: 'g2', panes: ['c'], active: 'c', dir: 'row' },
  ],
  active: 'g1',
}

test('reconcile drops gone shells', () => {
  assert.deepEqual(reconcile(two, ['a', 'c']), {
    groups: [
      { id: 'g1', panes: ['a'], active: 'a', dir: 'row' },
      { id: 'g2', panes: ['c'], active: 'c', dir: 'row' },
    ],
    active: 'g1',
  })
  assert.deepEqual(reconcile(two, ['c']), { groups: [{ id: 'g2', panes: ['c'], active: 'c', dir: 'row' }], active: 'g2' })
  assert.deepEqual(reconcile(two, []), { groups: [], active: '' })
})

test('a shell in no group gets its own group', () => {
  assert.deepEqual(reconcile(undefined, ['a', 'b']), {
    groups: [
      { id: 'g1', panes: ['a'], active: 'a', dir: 'row' },
      { id: 'g2', panes: ['b'], active: 'b', dir: 'row' },
    ],
    active: 'g1',
  })
  const after = reconcile(two, ['a', 'b', 'c', 'd'])
  assert.deepEqual(after.groups.at(-1), { id: 'g3', panes: ['d'], active: 'd', dir: 'row' })
  assert.equal(after.active, 'g1')
})

test('a group keeps its direction, and one saved with none is a row', () => {
  const stacked: ProjectTerms = { groups: [{ id: 'g1', panes: ['a', 'b'], active: 'a', dir: 'column' }], active: 'g1' }
  assert.equal(reconcile(stacked, ['a', 'b']).groups[0].dir, 'column')
  const old = { groups: [{ id: 'g1', panes: ['a'], active: 'a' }], active: 'g1' } as unknown as ProjectTerms
  assert.equal(reconcile(old, ['a']).groups[0].dir, 'row')
})

test('a new terminal is a new group', () => {
  const t = addGroup(two, 'd')
  assert.deepEqual(t.groups.at(-1), { id: 'g3', panes: ['d'], active: 'd', dir: 'row' })
  assert.equal(t.active, 'g3')
  assert.deepEqual(addGroup({ groups: [], active: '' }, 'a'), { groups: [{ id: 'g1', panes: ['a'], active: 'a', dir: 'row' }], active: 'g1' })
})

test('split stops at four panes', () => {
  const t = split(two, 'd')
  assert.deepEqual(t.groups[0], { id: 'g1', panes: ['a', 'b', 'd'], active: 'd', dir: 'row' })
  const full = split(t, 'e')
  assert.equal(full.groups[0].panes.length, MAX_PANES)
  assert.equal(split(full, 'f'), full)
  assert.deepEqual(split(select(two, 'a'), 'd').groups[0].panes, ['a', 'd', 'b'])
})

test('a split sets the direction of the whole group', () => {
  const stacked = split(two, 'd', 'column')
  assert.equal(stacked.groups[0].dir, 'column')
  assert.equal(stacked.groups[1].dir, 'row', 'the other groups stay')
  assert.equal(split(stacked, 'e', 'row').groups[0].dir, 'row')
})

test('a split with no group shown makes a group', () => {
  assert.deepEqual(split({ groups: [], active: '' }, 'a', 'column'), { groups: [{ id: 'g1', panes: ['a'], active: 'a', dir: 'row' }], active: 'g1' })
})

test('closing the last pane closes the group', () => {
  const t = selectGroup(two, 'g2')
  assert.deepEqual(closePane(t, 'c'), { groups: [two.groups[0]], active: 'g1' })
  assert.deepEqual(closePane(two, 'b').groups[0], { id: 'g1', panes: ['a'], active: 'a', dir: 'row' })
  assert.equal(closePane(two, 'c').active, 'g1')
})

test('selecting a group shows it, and an unknown one changes nothing', () => {
  assert.equal(selectGroup(two, 'g2').active, 'g2')
  assert.equal(selectGroup(two, 'g9'), two)
  assert.equal(selectGroup(two, 'g1'), two)
})

test('the label is the process name, else Terminal and its number', () => {
  assert.equal(terminalLabel('vim', 2), 'vim')
  assert.equal(terminalLabel('', 3), 'Terminal 3')
})

test("a shell's number ends its id, and the first has none", () => {
  assert.equal(terminalNumber('main/shell'), 1)
  assert.equal(terminalNumber('main/shell-4'), 4)
  assert.equal(terminalNumber('my-app-2/shell'), 1)
})

test("a folder is shown by its last part", () => {
  assert.equal(folderName('/Users/sam/code/widgets'), 'widgets')
  assert.equal(folderName('/Users/sam/code/widgets/'), 'widgets')
  assert.equal(folderName('/'), '/')
})

test('the list shows from two terminals in the group', () => {
  assert.equal(showsList(two.groups[1]), false)
  assert.equal(showsList(two.groups[0]), true)
})

test('two quick splits both land in the group, with an older shells event between them', () => {
  let t = withShells(undefined, ['p/shell'])
  t = split(t, 'p/shell-2')
  // Sent before shell-2 started, it lists only the first shell; shell-2 is kept as just started.
  t = prune(t, 'p', new Set(['p/shell', 'p/shell-2']))
  t = split(withShells(t, ['p/shell']), 'p/shell-3')
  assert.deepEqual(t, { groups: [{ id: 'g1', panes: ['p/shell', 'p/shell-2', 'p/shell-3'], active: 'p/shell-3', dir: 'row' }], active: 'g1' })
})

test("prune drops the project's gone shells and keeps other projects'", () => {
  const saved: ProjectTerms = { groups: [{ id: 'g1', panes: ['p/shell', 'q/shell', 'p/shell-2'], active: 'p/shell-2', dir: 'row' }], active: 'g1' }
  assert.deepEqual(prune(saved, 'p', new Set(['p/shell'])), { groups: [{ id: 'g1', panes: ['p/shell', 'q/shell'], active: 'q/shell', dir: 'row' }], active: 'g1' })
})

test('withShells adds new shells and drops none', () => {
  assert.deepEqual(withShells(two, ['c', 'd']), { groups: [...two.groups, { id: 'g3', panes: ['d'], active: 'd', dir: 'row' }], active: 'g1' })
})
