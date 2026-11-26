import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ACCESS_HINT, ACCESS_LABEL, ACCESS_NAME, accessDisabled, newThreadAccess, showsAccessIcon } from './access.ts'

test('labels', () => {
  assert.deepEqual(ACCESS_LABEL, { read: 'Read', supervised: 'Write · Supervised', full: 'Write · Full access' })
  assert.deepEqual(ACCESS_NAME, { read: 'Read', supervised: 'Supervised', full: 'Full access' })
  assert.equal(ACCESS_HINT.supervised, 'Your Claude allow rules apply; other tools ask in the chat')
  assert.equal(ACCESS_HINT.full, 'Every tool runs without asking')
})

test('an unset default access is Supervised', () => {
  assert.equal(newThreadAccess(''), 'supervised')
  assert.equal(newThreadAccess('full'), 'full')
})

test('no access icon at the default', () => {
  assert.equal(showsAccessIcon('supervised', ''), false, 'the default is Supervised when unset')
  assert.equal(showsAccessIcon('read', 'read'), false)
  assert.equal(showsAccessIcon('full', 'full'), false)
})

test('an icon when the access differs from the default', () => {
  assert.equal(showsAccessIcon('read', ''), true)
  assert.equal(showsAccessIcon('full', ''), true)
  assert.equal(showsAccessIcon('supervised', 'read'), true)
  assert.equal(showsAccessIcon('supervised', 'full'), true)
})

test('accessDisabled during a turn and an approval', () => {
  assert.equal(accessDisabled(false, false), '')
  assert.equal(accessDisabled(true, false), 'Change access when the turn ends')
  assert.equal(accessDisabled(false, true), 'Change access when the turn ends')
  assert.equal(accessDisabled(true, true), 'Change access when the turn ends')
  assert.equal(accessDisabled(false, false, true), 'Change access when the subagents end', 'a change restarts Claude, which stops them')
})
