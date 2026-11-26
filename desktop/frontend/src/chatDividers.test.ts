import assert from 'node:assert/strict'
import { test } from 'node:test'
import { forkDividers } from './chatDividers.ts'
import type { TimelineItem } from './chatTimeline.ts'
import type { Session } from './types.ts'

const reply = (uuid: string): TimelineItem => ({ kind: 'reply', key: `reply-${uuid}`, at: 0, id: uuid, uuid, text: uuid, streaming: false })
const user = (uuid: string): TimelineItem => ({ kind: 'user', key: `user-${uuid}`, at: 0, id: uuid, uuid, text: uuid, files: [], forkAt: '' })
const thread = (id: string, over: Partial<Session> = {}) => ({ id, forkOf: '', forkAt: '', ...over }) as Session
const shape = (items: TimelineItem[]) => items.map((i) => (i.kind === 'forkedFrom' ? `from:${i.source}` : i.kind === 'forked' ? `forked:${i.fork}` : 'uuid' in i ? i.uuid : i.kind))

test('a fork shows its divider after the copied reply', () => {
  const fork = thread('p/b', { forkOf: 'p/a', forkAt: 'r1' })
  const items = [user('u1'), reply('r1'), user('u2'), reply('r2')]
  assert.deepEqual(shape(forkDividers(fork, [thread('p/a'), fork], items)), ['u1', 'r1', 'from:p/a', 'u2', 'r2'])
})

test('a fork whose copied reply is missing shows its divider at the top', () => {
  const fork = thread('p/b', { forkOf: 'p/a', forkAt: 'gone' })
  assert.deepEqual(shape(forkDividers(fork, [fork], [user('u1')])), ['from:p/a', 'u1'])
})

test('the source shows conversation forked at that reply', () => {
  const source = thread('p/a')
  const forks = [thread('p/b', { forkOf: 'p/a', forkAt: 'r1' }), thread('p/c', { forkOf: 'p/a', forkAt: 'r2' }), thread('p/d', { forkOf: 'p/x', forkAt: 'r1' })]
  const items = [user('u1'), reply('r1'), reply('r2')]
  assert.deepEqual(shape(forkDividers(source, [source, ...forks], items)), ['u1', 'r1', 'forked:p/b', 'r2', 'forked:p/c'])
})

test('a source whose reply is gone shows none', () => {
  const source = thread('p/a')
  const fork = thread('p/b', { forkOf: 'p/a', forkAt: 'gone' })
  const items = [user('u1'), reply('r1')]
  assert.equal(forkDividers(source, [source, fork], items).length, items.length)
})
