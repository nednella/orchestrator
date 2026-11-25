import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ACCESS_LABEL, accessOf, choiceOf } from './access.ts'
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
})
