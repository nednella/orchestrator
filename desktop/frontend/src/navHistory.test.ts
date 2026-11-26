import assert from 'node:assert/strict'
import { test } from 'node:test'
import { back, closeAgent, current, forward, goneAgent, NO_HISTORY, push, viewedAgent } from './navHistory.ts'
import type { History, View } from './navHistory.ts'

const home: View = { kind: 'home', project: 'p' }
const thread = (id: string): View => ({ kind: 'thread', id })
const agent = (thread: string, agent: string): View => ({ kind: 'agent', thread, agent })
const of = (...views: View[]): History => views.reduce(push, NO_HISTORY)

test('back returns to the parent', () => {
  const h = back(of(thread('p/lead'), thread('p/child')))
  assert.deepEqual(current(h), thread('p/lead'))
  assert.deepEqual(current(forward(h)), thread('p/child'))
})

test('back and forward stop at the ends', () => {
  const h = of(home, thread('p/a'))
  assert.equal(forward(h), h)
  assert.equal(back(back(h)).at, 0)
  assert.equal(back(NO_HISTORY), NO_HISTORY)
})

test('a push after back drops forward', () => {
  const h = push(back(back(of(home, thread('p/a'), thread('p/b')))), thread('p/c'))
  assert.deepEqual(h.views, [home, thread('p/c')])
  assert.equal(forward(h), h)
})

test('the same view twice is one entry', () => {
  const h = of(thread('p/a'))
  assert.equal(push(h, thread('p/a')), h)
  assert.equal(of(thread('p/a'), agent('p/a', 'a1'), agent('p/a', 'a1')).views.length, 2)
  assert.equal(of(home, { kind: 'home', project: 'q' }).views.length, 2, "another project's home is another view")
})

test('history keeps fifty', () => {
  const h = of(...Array.from({ length: 60 }, (_, i) => thread(`p/${i}`)))
  assert.equal(h.views.length, 50)
  assert.equal(h.at, 49)
  assert.deepEqual(h.views[0], thread('p/10'), 'the oldest go')
  assert.deepEqual(current(h), thread('p/59'))
})

const viewing = of(thread('p/a'), agent('p/a', 'a1'))

test('a view shows only in its own thread', () => {
  assert.equal(viewedAgent(viewing, 'p/a'), 'a1')
  assert.equal(viewedAgent(viewing, 'p/b'), null)
  assert.equal(viewedAgent(of(thread('p/a')), 'p/a'), null)
  assert.equal(viewedAgent(NO_HISTORY, 'p/a'), null)
})

test('closing a thread leaves the view of another', () => {
  const closed = closeAgent(viewing, 'p/a')
  assert.equal(viewedAgent(closed, 'p/a'), null)
  assert.deepEqual(closed.views, [thread('p/a')], "the thread's chat takes the place of its subagent's view, once")
  assert.equal(closeAgent(viewing, 'p/b'), viewing)
  assert.deepEqual(closeAgent(of(home, agent('p/a', 'a1')), 'p/a').views, [home, thread('p/a')])
})

test('a view of a thread not on screen closes', () => {
  const left = push(viewing, thread('p/b'))
  assert.equal(viewedAgent(left, 'p/a'), null, 'the owner left the thread')
  assert.equal(viewedAgent(push(left, thread('p/a')), 'p/a'), null, 'it opens again on its chat')
  assert.equal(viewedAgent(back(left), 'p/a'), 'a1', 'back opens the view again')
})

test('a view whose subagent the loaded chat does not hold closes', () => {
  assert.equal(goneAgent(viewing, 'p/a', true, true), viewing)
  assert.equal(viewedAgent(goneAgent(viewing, 'p/a', true, false), 'p/a'), null)
  assert.equal(goneAgent(viewing, 'p/a', false, false), viewing, 'a chat that loads may not hold it yet')
  assert.equal(goneAgent(viewing, 'p/b', true, false), viewing, "another thread's chat says nothing of it")
})
