import assert from 'node:assert/strict'
import { test } from 'node:test'
import { registerModelJump } from '../modelJump.ts'
import { modelActions } from './models.ts'

test('each model has a number shortcut that picks it in the open menu and does nothing when none is open', () => {
  const actions = modelActions()
  assert.deepEqual(actions.map((x) => x.shortcut?.key), ['1', '2', '3', '4'])
  actions[1].run()
  const jumps: number[] = []
  const unregister = registerModelJump((n) => void jumps.push(n))
  actions[1].run()
  actions[3].run()
  unregister()
  actions[0].run()
  assert.deepEqual(jumps, [2, 4])
})
