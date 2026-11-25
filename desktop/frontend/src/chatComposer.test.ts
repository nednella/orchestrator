import assert from 'node:assert/strict'
import { test } from 'node:test'
import { composerAction, escapeAction, lastTurnEnd, sendText, unseenDrafts } from './chatComposer.ts'
import type { ChatEvent, Pending } from './types.ts'

type Draft = Omit<ChatEvent, 'seq' | 'at'>

const log = (...drafts: Draft[]): ChatEvent[] => drafts.map((d, i) => ({ at: (i + 1) * 1000, ...d, seq: i + 1 }))
const draft = (text: string): Draft => ({ kind: 'draft', text })
const user = (id: string, text: string): Draft => ({ kind: 'user', id, text })

const waiting: Pending = { id: 'p/abc', request: 'req-1', tool: 'Bash', summary: 'npm install', kind: 'tool' }
const idle = { turnOpen: false, pending: null }
const running = { turnOpen: true, pending: null }
const approving = { turnOpen: true, pending: waiting }

test('escapeAction: a picker open while running closes the picker', () => {
  assert.equal(escapeAction({ overlay: false, picker: true, turnOpen: true, stopping: false }), 'close-picker')
})

test('escapeAction: an overlay closes first', () => {
  assert.equal(escapeAction({ overlay: true, picker: true, turnOpen: true, stopping: false }), 'close-overlay')
})

test('escapeAction: a running turn is interrupted', () => {
  assert.equal(escapeAction({ overlay: false, picker: false, turnOpen: true, stopping: false }), 'interrupt')
})

test('escapeAction: a turn already stopping is not interrupted again', () => {
  assert.equal(escapeAction({ overlay: false, picker: false, turnOpen: true, stopping: true }), 'none')
})

test('escapeAction: an idle thread blurs the composer', () => {
  assert.equal(escapeAction({ overlay: false, picker: false, turnOpen: false, stopping: false }), 'blur')
})

test('composerAction: an idle thread sends', () => {
  assert.equal(composerAction(idle, false), 'send')
  assert.equal(composerAction(idle, true), 'send')
})

test('composerAction: a running turn queues a message, and stops with an empty draft', () => {
  assert.equal(composerAction(running, false), 'queue')
  assert.equal(composerAction(running, true), 'stop')
})

test('composerAction: a message while an approval waits denies it', () => {
  assert.equal(composerAction(approving, false), 'deny')
  assert.equal(composerAction(approving, true), 'stop')
})

test('unseenDrafts: drafts after the last user message and the seen mark', () => {
  const events = log(user('u1', 'Hi'), draft('one'), draft('two'))
  assert.deepEqual(
    unseenDrafts(events, 0).map((e) => e.text),
    ['one', 'two'],
  )
  assert.deepEqual(
    unseenDrafts(events, 2).map((e) => e.text),
    ['two'],
  )
  assert.deepEqual(unseenDrafts(events, 3), [])
})

test('unseenDrafts: a draft followed by a user message is consumed', () => {
  assert.deepEqual(unseenDrafts(log(draft('Fix the checks.'), user('u1', 'Fix the checks.')), 0), [])
})

test('unseenDrafts: a resumed thread with no mark shows only the drafts after its last message', () => {
  const events = log(draft('old'), user('u1', 'old'), { kind: 'turn_end' }, { kind: 'exit' }, draft('new'))
  assert.deepEqual(
    unseenDrafts(events, 0).map((e) => e.text),
    ['new'],
  )
})

test('sendText keeps leading indent', () => {
  assert.equal(sendText('    indented code\n  \n\t'), '    indented code')
  assert.equal(sendText('  two\nlines  '), '  two\nlines')
})

test('lastTurnEnd is the seq of the latest turn end or exit', () => {
  assert.equal(lastTurnEnd(log(user('u1', 'Hi'))), 0)
  assert.equal(lastTurnEnd(log(user('u1', 'Hi'), { kind: 'turn_end' }, user('u2', 'Again'))), 2)
  assert.equal(lastTurnEnd(log(user('u1', 'Hi'), { kind: 'turn_end' }, { kind: 'exit' })), 3)
})
