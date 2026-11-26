import assert from 'node:assert/strict'
import { test } from 'node:test'
import { makeInputBatch } from './inputBatch.ts'

function frames() {
  const queued: (() => void)[] = []
  return { schedule: (run: () => void) => void queued.push(run), next: () => queued.splice(0).forEach((run) => run()) }
}

test('keys in one frame are one write', () => {
  const sent: string[] = []
  const frame = frames()
  const write = makeInputBatch((data) => sent.push(data), frame.schedule)
  write('l')
  write('s')
  write('\x1b[<64;10;5M')
  assert.deepEqual(sent, [])
  frame.next()
  assert.deepEqual(sent, ['ls\x1b[<64;10;5M'])
})

test('the next frame starts a new write', () => {
  const sent: string[] = []
  const frame = frames()
  const write = makeInputBatch((data) => sent.push(data), frame.schedule)
  write('a')
  frame.next()
  write('b')
  write('c')
  frame.next()
  frame.next()
  assert.deepEqual(sent, ['a', 'bc'])
})
