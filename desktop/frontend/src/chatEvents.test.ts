import assert from 'node:assert/strict'
import { test } from 'node:test'
import { EMPTY_CHAT, mergeEvents } from './chatEvents.ts'
import type { ChatEvent } from './types.ts'

const logged = (seq: number, kind: ChatEvent['kind'] = 'tool_use'): ChatEvent => ({ seq, at: seq * 1000, kind })
const delta = (id: string, text: string): ChatEvent => ({ seq: 0, at: 0, kind: 'delta', id, text })
const seqs = (events: ChatEvent[]) => events.map((e) => e.seq)

test('merge drops doubles and reports a gap', () => {
  const first = mergeEvents(EMPTY_CHAT, [logged(1), logged(2), logged(3)])
  assert.deepEqual(seqs(first.state.events), [1, 2, 3])
  assert.equal(first.state.last, 3)
  assert.equal(first.gapAfter, null)

  const doubled = mergeEvents(first.state, [logged(2), logged(3), logged(4)])
  assert.deepEqual(seqs(doubled.state.events), [1, 2, 3, 4])
  assert.equal(doubled.gapAfter, null)

  const again = mergeEvents(doubled.state, [logged(3), logged(4)])
  assert.equal(again.state, doubled.state, 'nothing new keeps the state as it was')

  const gapped = mergeEvents(doubled.state, [logged(6), logged(7)])
  assert.equal(gapped.gapAfter, 4)
  assert.deepEqual(seqs(gapped.state.events), [1, 2, 3, 4], 'events past the gap wait for the history')
  assert.equal(gapped.state.last, 4)

  const filled = mergeEvents(gapped.state, [logged(5), logged(6), logged(7)])
  assert.deepEqual(seqs(filled.state.events), [1, 2, 3, 4, 5, 6, 7])
  assert.equal(filled.gapAfter, null)
})

test('deltas pass through without a seq', () => {
  const { state, gapAfter } = mergeEvents(EMPTY_CHAT, [logged(1, 'user'), delta('msg_1', 'Hel'), delta('msg_1', 'lo'), delta('msg_2', 'x')])
  assert.deepEqual(seqs(state.events), [1])
  assert.equal(state.last, 1)
  assert.equal(gapAfter, null)
  assert.deepEqual(state.live, { msg_1: 'Hello', msg_2: 'x' })

  const replied = mergeEvents(state, [{ seq: 2, at: 2000, kind: 'message', id: 'msg_1', text: 'Hello' }])
  assert.deepEqual(replied.state.live, { msg_2: 'x' })
  assert.deepEqual(state.live, { msg_1: 'Hello', msg_2: 'x' }, 'the state merged into is not changed')

  const stopped = mergeEvents(replied.state, [{ seq: 3, at: 3000, kind: 'turn_end', error: true }])
  assert.deepEqual(stopped.state.live, {}, 'a reply cut off at the turn end is never logged')
})
