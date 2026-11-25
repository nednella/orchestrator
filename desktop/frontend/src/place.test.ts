import assert from 'node:assert/strict'
import { test } from 'node:test'
import { defaultPlace, PLACE_LABEL } from './place.ts'

test('read defaults to the project folder', () => {
  assert.equal(defaultPlace('read', false), 'project')
  assert.equal(PLACE_LABEL.project, 'Project folder')
})

test('write and issues default to a worktree', () => {
  assert.equal(defaultPlace('supervised', false), 'worktree')
  assert.equal(defaultPlace('full', false), 'worktree')
  assert.equal(defaultPlace('read', true), 'worktree')
  assert.equal(PLACE_LABEL.worktree, 'New worktree')
})
