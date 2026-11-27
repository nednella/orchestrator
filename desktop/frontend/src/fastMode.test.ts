import assert from 'node:assert/strict'
import { test } from 'node:test'
import { fastCommand, fastCommands } from './fastMode.ts'

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
