import assert from 'node:assert/strict'
import { test } from 'node:test'
import { activeView, addView, NO_VIEWS, removeView, renameView, selectView, storedViews, updateView, viewNameError } from './listViews.ts'
import { DEFAULT_ISSUE_PREFS, fixIssuePrefs, normalIssuePrefs } from './issueList.ts'

const mine = { ...DEFAULT_ISSUE_PREFS, involvement: 'assigned' as const }

test('a name must be there, short and not taken', () => {
  const saved = addView(NO_VIEWS, 'a', 'Assigned to me', mine)
  assert.equal(viewNameError('  ', saved.views), 'Enter a name.')
  assert.equal(viewNameError('x'.repeat(41), saved.views), 'Use 40 characters or fewer.')
  assert.equal(viewNameError('assigned TO me', saved.views), 'A view with this name exists.')
  assert.equal(viewNameError('assigned to me', saved.views, 'a'), '', 'a view may keep its own name')
  assert.equal(viewNameError('Bugs', saved.views), '')
})

test('a saved view is selected, and modified once the list moves off it', () => {
  const saved = addView(NO_VIEWS, 'a', 'Mine', mine)
  assert.deepEqual(activeView(saved, mine, normalIssuePrefs), { view: saved.views[0], modified: false })
  const moved = { ...mine, type: 'bug' }
  assert.deepEqual(activeView(saved, moved, normalIssuePrefs), { view: saved.views[0], modified: true })
})

test('label order does not make a view modified', () => {
  const saved = addView(NO_VIEWS, 'a', 'Two', { ...DEFAULT_ISSUE_PREFS, labels: ['b', 'a'] })
  assert.equal(activeView(saved, { ...DEFAULT_ISSUE_PREFS, labels: ['a', 'b'] }, normalIssuePrefs).modified, false)
})

test('a list that matches a view without one picked shows that view', () => {
  const picked = selectView(addView(NO_VIEWS, 'a', 'Mine', mine), '')
  assert.equal(activeView(picked, mine, normalIssuePrefs).view?.id, 'a')
  assert.equal(activeView(picked, DEFAULT_ISSUE_PREFS, normalIssuePrefs).view, null)
})

test('rename, update and delete act on one view', () => {
  let saved = addView(addView(NO_VIEWS, 'a', 'One', mine), 'b', 'Two', DEFAULT_ISSUE_PREFS)
  saved = renameView(saved, 'a', ' Uno ')
  assert.equal(saved.views[0].name, 'Uno')
  saved = updateView(saved, 'a', { ...mine, sort: 'created' })
  assert.equal(saved.views[0].prefs.sort, 'created')
  assert.equal(saved.views[1].prefs.sort, 'updated')
  saved = removeView(selectView(saved, 'b'), 'b')
  assert.deepEqual(saved.views.map((v) => v.id), ['a'])
  assert.equal(saved.selected, '')
})

test('stored views of another shape are none, and a view keeps only what it knows', () => {
  assert.deepEqual(storedViews(7, fixIssuePrefs), NO_VIEWS)
  const read = storedViews({ views: [{ id: 'a', name: 'A', prefs: { sort: 'nope', labels: [1, 'x'] } }, { id: 3 }], selected: 'gone' }, fixIssuePrefs)
  assert.equal(read.views.length, 1)
  assert.deepEqual(read.views[0].prefs, { ...DEFAULT_ISSUE_PREFS, labels: ['x'] })
  assert.equal(read.selected, '')
})
