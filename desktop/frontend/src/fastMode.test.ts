import assert from 'node:assert/strict'
import { test } from 'node:test'
import { fastCommand, fastCommands, fastModeNote } from './fastMode.ts'
import type { ChatEvent } from './types.ts'

test('/fast alone turns fast mode over', () => {
  assert.equal(fastCommand('/fast', false), true)
  assert.equal(fastCommand('/fast', true), false)
  assert.equal(fastCommand('  /FAST \n', false), true)
})

test('/fast on and /fast off set it, whatever it was', () => {
  assert.equal(fastCommand('/fast on', true), true)
  assert.equal(fastCommand('/fast on', false), true)
  assert.equal(fastCommand('/fast off', true), false)
  assert.equal(fastCommand('/fast  Off', false), false)
})

test('a message that is not the command goes to Claude', () => {
  for (const text of ['', 'fast', 'make it /fast', '/faster', '/fast maybe', '/fast on please', '/fast\non and more']) {
    assert.equal(fastCommand(text, false), null, text)
  }
})

test('the slash menu lists /fast', () => {
  assert.deepEqual(fastCommands().map((c) => c.name), ['fast'])
})

const init = (text?: string): ChatEvent => ({ kind: 'init', id: 's', ...(text === undefined ? {} : { text }) }) as ChatEvent

test('a thread told to run fast says why Claude runs at normal speed', () => {
  assert.equal(fastModeNote([init('extra_usage_disabled')], true), 'Extra usage is off for this account, so Claude runs at normal speed.')
  assert.equal(fastModeNote([init('some_new_reason')], true), 'some new reason, so Claude runs at normal speed.')
})

test('it says nothing when fast mode is off, works, or Claude gives no reason', () => {
  assert.equal(fastModeNote([init('extra_usage_disabled')], false), '')
  assert.equal(fastModeNote([init()], true), '')
  assert.equal(fastModeNote([], true), '')
  assert.equal(fastModeNote([init('sdk_opt_in_required')], true), '')
})

test('the latest run is the one that counts', () => {
  assert.equal(fastModeNote([init('extra_usage_disabled'), init()], true), '')
  assert.equal(fastModeNote([init(), init('free')], true), 'Fast mode needs a paid plan, so Claude runs at normal speed.')
})
