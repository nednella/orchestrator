import assert from 'node:assert/strict'
import { test } from 'node:test'
import { defaultPlace, PLACE_LABEL, placeFor } from './place.ts'

test('read defaults to the project folder', () => {
  assert.equal(defaultPlace('read', false), 'project')
})

test('write and issues default to a worktree', () => {
  assert.equal(defaultPlace('supervised', false), 'worktree')
  assert.equal(defaultPlace('full', false), 'worktree')
  assert.equal(defaultPlace('read', true), 'worktree')
})

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
