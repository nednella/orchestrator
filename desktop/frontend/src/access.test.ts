import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ACCESS_HINT, ACCESS_LABEL, accessDisabled, accessOf, choiceOf } from './access.ts'
import type { Access } from './types.ts'

test('accessOf and choiceOf round-trip', () => {
  for (const access of ['read', 'supervised', 'full'] as Access[]) assert.equal(accessOf(choiceOf(access)), access)
  assert.deepEqual(choiceOf('read'), { level: 'read', write: 'supervised' })
  assert.deepEqual(choiceOf('full'), { level: 'write', write: 'full' })
  assert.equal(accessOf({ level: 'read', write: 'full' }), 'read', 'Read wins over the Write choice under it')
  assert.equal(accessOf({ ...choiceOf('read'), level: 'write' }), 'supervised', 'Write after Read is Supervised')
})

test('labels', () => {
  assert.deepEqual(ACCESS_LABEL, { read: 'Read', supervised: 'Write · Supervised', full: 'Write · Full access' })
  assert.equal(ACCESS_HINT.supervised, 'Your Claude allow rules apply; other tools ask in the chat')
  assert.equal(ACCESS_HINT.full, 'Every tool runs without asking')
})

test('accessDisabled during a turn and an approval', () => {
  assert.equal(accessDisabled(false, false), '')
  assert.equal(accessDisabled(true, false), 'Change access when the turn ends')
  assert.equal(accessDisabled(false, true), 'Change access when the turn ends')
  assert.equal(accessDisabled(true, true), 'Change access when the turn ends')
  assert.equal(accessDisabled(false, false, true), 'Change access when the subagents end', 'a change restarts Claude, which stops them')
})
