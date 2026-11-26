import assert from 'node:assert/strict'
import { test } from 'node:test'
import { makeInputBatch } from './inputBatch.ts'

function timers() {
  const queued: (() => void)[] = []
  return { schedule: (run: () => void) => void queued.push(run), fire: () => queued.splice(0).forEach((run) => run()) }
}

test('a lone key is written at once', () => {
  const sent: string[] = []
  const timer = timers()
  const write = makeInputBatch((data) => sent.push(data), timer.schedule)
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
  const write = makeInputBatch((data) => sent.push(data), timer.schedule)
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
