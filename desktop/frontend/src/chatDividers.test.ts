import assert from 'node:assert/strict'
import { test } from 'node:test'
import { forkDividers, lineageDivider } from './chatDividers.ts'
import type { TimelineItem } from './chatTimeline.ts'

const reply = (uuid: string): TimelineItem => ({ kind: 'reply', key: `reply-${uuid}`, at: 0, id: uuid, uuid, text: uuid, streaming: false })
const user = (uuid: string): TimelineItem => ({ kind: 'user', key: `user-${uuid}`, at: 0, id: uuid, uuid, text: uuid, files: [], forkAt: '' })
const work = (key: string): TimelineItem => ({ kind: 'work', key, startedAt: 0, endedAt: 1, calls: [], steps: [] })
const thread = (id: string, forkOf = '', forkAt = '', title = id) => ({ id, forkOf, forkAt, title })
const shape = (items: TimelineItem[]) => items.map((i) => (i.kind === 'forkedFrom' ? `from:${i.source}` : i.kind === 'forked' ? `forked:${i.fork}` : i.kind === 'work' ? i.key : 'uuid' in i ? i.uuid : i.kind))

test('a fork shows its divider after the copied reply', () => {
  const fork = thread('p/b', 'p/a', 'r1')
  assert.deepEqual(shape(forkDividers(fork, [], [user('u1'), reply('r1'), user('u2'), reply('r2')])), ['u1', 'r1', 'from:p/a', 'u2', 'r2'])
})

test('a fork whose copied reply is missing shows its divider at the top', () => {
  assert.deepEqual(shape(forkDividers(thread('p/b', 'p/a', 'gone'), [], [user('u1')])), ['from:p/a', 'u1'])
})

test('the source shows conversation forked at that reply', () => {
  const forks = [thread('p/b', 'p/a', 'r1'), thread('p/c', 'p/a', 'r2')]
  assert.deepEqual(shape(forkDividers(thread('p/a'), forks, [user('u1'), reply('r1'), reply('r2')])), ['u1', 'r1', 'forked:p/b', 'r2', 'forked:p/c'])
})

test('a source whose reply is gone shows none', () => {
  const items = [user('u1'), reply('r1')]
  assert.equal(forkDividers(thread('p/a'), [thread('p/b', 'p/a', 'gone')], items).length, items.length)
})

test('a reply keeps the work rows that follow it next to it', () => {
  const items = [user('u1'), reply('r1'), work('w1'), work('w2'), user('u2')]
  assert.deepEqual(shape(forkDividers(thread('p/a'), [thread('p/b', 'p/a', 'r1')], items)), ['u1', 'r1', 'w1', 'w2', 'forked:p/b', 'u2'])
  assert.deepEqual(shape(forkDividers(thread('p/b', 'p/a', 'r1'), [], items)), ['u1', 'r1', 'w1', 'w2', 'from:p/a', 'u2'])
})

test('a mark after the last reply and its work goes at the end', () => {
  assert.deepEqual(shape(forkDividers(thread('p/a'), [thread('p/b', 'p/a', 'r1')], [reply('r1'), work('w1')])), ['r1', 'w1', 'forked:p/b'])
})

test('several forks at one reply each get a divider with their title', () => {
  const forks = [thread('p/b', 'p/a', 'r1', 'first'), thread('p/c', 'p/a', 'r1', 'second')]
  const out = forkDividers(thread('p/a'), forks, [reply('r1')])
  assert.deepEqual(shape(out), ['r1', 'forked:p/b', 'forked:p/c'])
  assert.deepEqual(out.flatMap((i) => (i.kind === 'forked' ? [i.title, i.key] : [])), ['first', 'forked-p/b', 'second', 'forked-p/c'])
})

test('a fork of a fork shows both its own source mark and the marks of its forks', () => {
  const middle = thread('p/b', 'p/a', 'r1')
  const out = forkDividers(middle, [thread('p/c', 'p/b', 'r2')], [user('u1'), reply('r1'), user('u2'), reply('r2')])
  assert.deepEqual(shape(out), ['u1', 'r1', 'from:p/a', 'u2', 'r2', 'forked:p/c'])
})

test('a delegated child starts with the divider', () => {
  const lead = { id: 'p/lead', title: 'Fix login redirect' }
  assert.deepEqual(lineageDivider({ parent: 'p/lead' }, lead), { kind: 'lineage', key: 'lineage', parent: 'p/lead', title: 'Fix login redirect' })
  assert.equal(lineageDivider({ parent: 'p/gone' }, undefined), null, 'a child whose parent is gone has no way back')
})

test('a top-level thread has none', () => {
  assert.equal(lineageDivider({ parent: '' }, undefined), null)
})
