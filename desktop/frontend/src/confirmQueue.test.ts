import assert from 'node:assert/strict'
import { test } from 'node:test'
import { answerFor, check, current, enqueue, settle } from './confirmQueue.ts'
import type { ConfirmRequest } from './confirmQueue.ts'

const ask = (id: number, title: string): ConfirmRequest => ({ id, title, confirm: 'Delete', tone: 'danger' })

test('confirms are shown one at a time', () => {
  const q = enqueue(enqueue([], ask(1, 'Delete this note?')), ask(2, 'Delete this evidence?'))
  assert.equal(current(q)?.id, 1)
  assert.equal(current([]), undefined)
})

test('a second confirm waits for the first', () => {
  const q = enqueue(enqueue([], ask(1, 'Delete this note?')), ask(2, 'Delete this evidence?'))
  assert.equal(current(settle(q, 2))?.id, 1, 'settling a waiting one leaves the shown one')
  const next = settle(q, 1)
  assert.equal(current(next)?.id, 2)
  assert.equal(current(settle(next, 2)), undefined)
})

test('escape cancels', () => {
  assert.equal(answerFor('Escape'), false)
  assert.equal(answerFor('Enter'), undefined, 'Enter acts on the focused button')
  assert.equal(answerFor('a'), undefined)
})

test('ticking the box changes only that confirm', () => {
  const box = { label: 'Also delete its worktree', checked: false }
  const q = enqueue(enqueue([], { ...ask(1, 'Delete thread "a"?'), checkbox: box }), { ...ask(2, 'Delete thread "b"?'), checkbox: box })
  const ticked = check(q, 1, true)
  assert.equal(ticked[0].checkbox?.checked, true)
  assert.equal(ticked[1].checkbox?.checked, false)
  assert.equal(check(q, 3, true), q, 'no confirm of that id changes nothing')
})
