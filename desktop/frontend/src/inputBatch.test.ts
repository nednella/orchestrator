import assert from 'node:assert/strict'
import { test } from 'node:test'
import { makeInputBatch } from './inputBatch.ts'

function timers() {
  const queued = new Set<() => void>()
  return {
    schedule: (run: () => void) => {
      queued.add(run)
      return () => void queued.delete(run)
    },
    fire: () => {
      const due = [...queued]
      queued.clear()
      due.forEach((run) => run())
    },
    pending: () => queued.size,
  }
}

test('a lone key is written at once', () => {
  const sent: string[] = []
  const timer = timers()
  const { write } = makeInputBatch((data) => sent.push(data), timer.schedule)
  write('l')
  assert.deepEqual(sent, ['l'])
  timer.fire()
  timer.fire()
  write('s')
  assert.deepEqual(sent, ['l', 's'])
})

test('keys that follow quickly are joined', () => {
  const sent: string[] = []
  const timer = timers()
  const { write } = makeInputBatch((data) => sent.push(data), timer.schedule)
  write('a')
  write('b')
  write('\x1b[<64;10;5M')
  assert.deepEqual(sent, ['a'])
  timer.fire()
  assert.deepEqual(sent, ['a', 'b\x1b[<64;10;5M'])
  write('c')
  write('d')
  timer.fire()
  assert.deepEqual(sent, ['a', 'b\x1b[<64;10;5M', 'cd'])
  timer.fire()
  write('e')
  assert.deepEqual(sent, ['a', 'b\x1b[<64;10;5M', 'cd', 'e'])
})

test('after dispose the held keys are not sent', () => {
  const sent: string[] = []
  const timer = timers()
  const { write, dispose } = makeInputBatch((data) => sent.push(data), timer.schedule)
  write('a')
  write('b')
  dispose()
  assert.equal(timer.pending(), 0)
  timer.fire()
  write('c')
  timer.fire()
  assert.deepEqual(sent, ['a'])
})
