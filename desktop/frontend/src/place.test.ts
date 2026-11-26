import assert from 'node:assert/strict'
import { test } from 'node:test'
import { PLACE_LABEL, placeFor } from './place.ts'

test('unset is a new worktree', () => {
  assert.equal(placeFor('', false), 'worktree')
  assert.equal(PLACE_LABEL.worktree, 'New worktree')
})

test('current branch when set', () => {
  assert.equal(placeFor('project', false), 'project')
  assert.equal(placeFor('worktree', false), 'worktree')
  assert.equal(PLACE_LABEL.project, 'Current branch')
})

test('an issue is always a worktree', () => {
  assert.equal(placeFor('project', true), 'worktree')
  assert.equal(placeFor('', true), 'worktree')
})
