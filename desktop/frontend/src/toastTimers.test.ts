import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createToastTimers } from './toastTimers.ts'
import type { ToastClock } from './toastTimers.ts'
import type { Toast } from './toastPile.ts'

// A clock that moves only when the test says so.
function fakeClock() {
  let now = 0
  let next = 0
  const timers = new Map<number, { at: number; run: () => void }>()
  const clock: ToastClock = {
    now: () => now,
    setTimeout: (run, ms) => {
      timers.set(++next, { at: now + ms, run })
      return next
    },
    clearTimeout: (timer) => void timers.delete(timer as number),
  }
  const advance = (ms: number) => {
    const end = now + ms
    for (;;) {
      const due = [...timers.entries()].filter(([, t]) => t.at <= end).sort((a, b) => a[1].at - b[1].at)[0]
      if (!due) break
      timers.delete(due[0])
      now = due[1].at
      due[1].run()
    }
    now = end
  }
  return { clock, advance }
}

const toast = (key: number, more: Partial<Toast> = {}): Toast => ({ key, tone: 'info', text: `t${key}`, ...more })

function setup() {
  const { clock, advance } = fakeClock()
  const expired: number[] = []
  const timers = createToastTimers((key) => expired.push(key), 6000, clock)
  return { timers, advance, expired }
}

test('a toast expires after its time', () => {
  const { timers, advance, expired } = setup()
  timers.sync([toast(1)])
  advance(5999)
  assert.deepEqual(expired, [])
  advance(1)
  assert.deepEqual(expired, [1])
})

test('a sticky toast never expires', () => {
  const { timers, advance, expired } = setup()
  timers.sync([toast(1, { sticky: true })])
  advance(60_000)
  assert.deepEqual(expired, [])
})

test('every toast runs its own time', () => {
  const { timers, advance, expired } = setup()
  const first = toast(1)
  timers.sync([first])
  advance(2000)
  timers.sync([first, toast(2)])
  advance(4000)
  assert.deepEqual(expired, [1])
  advance(2000)
  assert.deepEqual(expired, [1, 2])
})

test('a paused pile holds every timer and goes on with what was left', () => {
  const { timers, advance, expired } = setup()
  const first = toast(1)
  timers.sync([first])
  advance(4000)
  timers.setPaused(true)
  timers.sync([first, toast(2)])
  advance(60_000)
  assert.deepEqual(expired, [])
  timers.setPaused(false)
  advance(2000)
  assert.deepEqual(expired, [1])
  advance(4000)
  assert.deepEqual(expired, [1, 2])
})

test('a replaced toast starts its time again', () => {
  const { timers, advance, expired } = setup()
  timers.sync([toast(1)])
  advance(5000)
  timers.sync([toast(1)])
  advance(5000)
  assert.deepEqual(expired, [])
  advance(1000)
  assert.deepEqual(expired, [1])
})

test('a toast taken out of the pile does not expire', () => {
  const { timers, advance, expired } = setup()
  timers.sync([toast(1), toast(2)])
  timers.sync([toast(2)])
  advance(6000)
  assert.deepEqual(expired, [2])
})

test('a toast that turns sticky stops running', () => {
  const { timers, advance, expired } = setup()
  timers.sync([toast(1)])
  timers.sync([toast(1, { sticky: true })])
  advance(60_000)
  assert.deepEqual(expired, [])
})

test('disposed timers never fire', () => {
  const { timers, advance, expired } = setup()
  timers.sync([toast(1)])
  timers.dispose()
  advance(60_000)
  assert.deepEqual(expired, [])
})
