import assert from 'node:assert/strict'
import { test } from 'node:test'
import { EMPTY_CHAT, chatFeed, mergeEvents } from './chatEvents.ts'
import type { ChatState } from './chatEvents.ts'
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

// feed builds a chat feed whose history reads wait until the test answers them, and whose later runs wait for frame().
function feed() {
  const reads: { after: number; answer(events: ChatEvent[]): void; fail(err: string): void }[] = []
  const runs: (() => void)[] = []
  const seen: { state: ChatState; errors: unknown[] } = { state: EMPTY_CHAT, errors: [] }
  const f = chatFeed(EMPTY_CHAT, {
    history: (after) => new Promise((answer, fail) => reads.push({ after, answer, fail })),
    later: (run) => {
      runs.push(run)
      return () => runs.splice(runs.indexOf(run), 1)
    },
    onState: (state) => (seen.state = state),
    onError: (err) => seen.errors.push(err),
  })
  const frame = () => runs.splice(0).forEach((run) => run())
  return { f, reads, runs, seen, frame }
}

const settle = () => new Promise((done) => setTimeout(done, 0))
const range = (from: number, to: number) => Array.from({ length: to - from + 1 }, (_, i) => logged(from + i))

test('live events wait for the first history read', async () => {
  const { f, reads, runs, seen } = feed()
  f.push([delta('msg_1', 'Hi'), logged(1), logged(2)])
  assert.equal(runs.length, 0, 'nothing is merged before the history')
  const loading = f.load()
  reads[0].answer([logged(1)])
  await loading
  assert.deepEqual(seqs(seen.state.events), [1, 2])
  assert.deepEqual(seen.state.live, { msg_1: 'Hi' })
})

test('a gap seen while a fill runs is filled after it', async () => {
  const { f, reads, seen, frame } = feed()
  const loading = f.load()
  reads[0].answer(range(1, 2))
  await loading
  f.push([logged(4)])
  frame()
  f.push([logged(6)])
  frame()
  assert.deepEqual(reads.map((r) => r.after), [0, 2], 'one fill at a time')
  reads[1].answer(range(3, 4))
  await settle()
  assert.deepEqual(reads.map((r) => r.after), [0, 2, 4])
  reads[2].answer(range(5, 6))
  await settle()
  assert.deepEqual(seqs(seen.state.events), [1, 2, 3, 4, 5, 6])
})

test('events held past the limit are read again', async () => {
  const { f, reads, seen } = feed()
  f.push([logged(2)])
  f.push(Array.from({ length: 2001 }, () => delta('msg_1', 'x')))
  const loading = f.load()
  reads[0].answer(range(1, 1))
  await loading
  assert.deepEqual(reads.map((r) => r.after), [0, 1], 'the history after the first read brings what was dropped')
  reads[1].answer(range(2, 2))
  await settle()
  assert.deepEqual(seqs(seen.state.events), [1, 2])
})

test('a failed first read stops the feed', async () => {
  const { f, reads, runs, seen } = feed()
  const loading = f.load()
  reads[0].fail('Could not read the chat')
  await loading
  assert.deepEqual(seen.errors, ['Could not read the chat'])
  f.push([logged(1)])
  assert.equal(runs.length, 0)
  assert.equal(seen.state, EMPTY_CHAT)
})
