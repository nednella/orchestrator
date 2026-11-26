import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { TimelineItem } from './chatTimeline.ts'
import { canRecall, historyOf, stepDown, stepUp } from './promptHistory.ts'

const user = (text: string, from?: string): TimelineItem => ({ kind: 'user', key: text + (from ?? ''), at: 0, id: text, uuid: text, text, files: [], forkAt: '', from })
const base = { key: 'ArrowUp' as const, modifier: false, composing: false, pending: false, attachments: 0, menuOpen: false, value: '', recall: null, thread: 't', onFirstLine: true, onLastLine: true }

test('up from empty recalls the newest', () => {
  assert.deepEqual(stepUp(['a', 'b'], null, 't'), { recall: { thread: 't', index: 1, text: 'b' }, text: 'b' })
  assert.deepEqual(stepUp(['a', 'b'], { thread: 't', index: 1, text: 'b' }, 't')?.text, 'a')
  assert.equal(stepUp([], null, 't'), null)
})

test('up stops at the oldest', () => {
  assert.equal(stepUp(['a', 'b'], { thread: 't', index: 0, text: 'a' }, 't'), null)
})

test('down past the newest clears', () => {
  assert.deepEqual(stepDown(['a', 'b'], { thread: 't', index: 0, text: 'a' }, 't')?.text, 'b')
  assert.deepEqual(stepDown(['a', 'b'], { thread: 't', index: 1, text: 'b' }, 't'), { recall: null, text: '' })
})

test('down with nothing recalled does nothing', () => {
  assert.equal(stepDown(['a'], null, 't'), null)
})

test('an edited recall leaves history', () => {
  const recall = { thread: 't', index: 0, text: 'hello' }
  assert.equal(canRecall({ ...base, value: 'hello', recall }), true)
  assert.equal(canRecall({ ...base, value: 'hello!', recall }), false)
  assert.equal(canRecall({ ...base, value: 'typed', recall: null }), false)
})

test('no recall with a modifier, during composition, with a pending question, with attachments or with a menu open', () => {
  assert.equal(canRecall(base), true)
  assert.equal(canRecall({ ...base, modifier: true }), false)
  assert.equal(canRecall({ ...base, composing: true }), false)
  assert.equal(canRecall({ ...base, pending: true }), false)
  assert.equal(canRecall({ ...base, attachments: 1 }), false)
  assert.equal(canRecall({ ...base, menuOpen: true }), false)
})

test('up only on the first line, down only on the last', () => {
  assert.equal(canRecall({ ...base, onFirstLine: false }), false)
  assert.equal(canRecall({ ...base, onLastLine: false }), true)
  assert.equal(canRecall({ ...base, key: 'ArrowDown', onLastLine: false }), false)
  assert.equal(canRecall({ ...base, key: 'ArrowDown', onFirstLine: false }), true)
})

test('equal prompts in a row count once', () => {
  assert.deepEqual(historyOf([user('a'), user('a'), user('b'), user('a')]), ['a', 'b', 'a'])
})

test('messages from another thread are left out', () => {
  const items: TimelineItem[] = [user('a'), user('from child', 'c1'), user('  '), { kind: 'childResult', key: 'r', at: 0, child: 'c1', text: 'done' }, user('b')]
  assert.deepEqual(historyOf(items), ['a', 'b'])
})

test('a recall in one thread is not active in another', () => {
  const recall = { thread: 't', index: 0, text: 'hello' }
  assert.equal(canRecall({ ...base, value: 'hello', recall, thread: 'u' }), false)
  assert.equal(stepDown(['a', 'b'], recall, 'u'), null)
  assert.equal(stepUp(['a', 'b'], recall, 'u')?.text, 'b')
})

test('a reply recalls without its quote', () => {
  assert.deepEqual(historyOf([user('> earlier line\n> second\n\nmy answer'), user('> only a quote')]), ['my answer', '> only a quote'])
})
