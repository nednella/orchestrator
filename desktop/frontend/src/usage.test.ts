import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { Limits } from './types.ts'
import { resetsIn, usageRows } from './usage.ts'

const now = Date.UTC(2026, 10, 26, 12, 0)
const limits = (windows: Limits['windows']): Limits => ({ windows, at: now, unsupported: false, error: '' })

test('percent left from get_usage', () => {
  const rows = usageRows(limits([
    { name: 'five_hour', label: 'Session', left: 74.5, resetsAt: now + 3 * 3_600_000 },
    { name: 'seven_day', label: 'Weekly', left: 60, resetsAt: now + (4 * 24 + 11) * 3_600_000 },
    { name: 'model:Fable', label: 'Weekly · Fable', left: 100, resetsAt: now + (4 * 24 + 11) * 3_600_000 },
    { name: 'spend', label: 'Monthly spend', left: 70, resetsAt: 0 },
  ]), now)
  assert.deepEqual(rows, [
    { label: 'Session', left: 75, resetsIn: '3h', tone: 'plain' },
    { label: 'Weekly', left: 60, resetsIn: '4d 11h', tone: 'plain' },
    { label: 'Weekly · Fable', left: 100, resetsIn: '4d 11h', tone: 'plain' },
    { label: 'Monthly spend', left: 70, resetsIn: '', tone: 'plain' },
  ])
})

test('stream utilization is scaled', () => {
  // A turn's rate_limit event says 0.96 of the week is used; Go keeps it as 4 left, to a hundredth.
  const [row] = usageRows(limits([{ name: 'seven_day', label: 'Weekly', left: 4.0000000000000036, resetsAt: now + 45 * 60_000 }]), now)
  assert.equal(row.left, 4)
  assert.equal(row.resetsIn, '45m')
  const [over] = usageRows(limits([{ name: 'five_hour', label: 'Session', left: -3, resetsAt: now - 1 }]), now)
  assert.deepEqual(over, { label: 'Session', left: 0, resetsIn: '', tone: 'danger' })
})

test('resets in', () => {
  const cases: [number, string][] = [
    [45 * 60_000, '45m'],
    [(2 * 60 + 5) * 60_000 + 59_000, '2h 5m'],
    [2 * 3_600_000, '2h'],
    [(3 * 24 + 4) * 3_600_000 + 30 * 60_000, '3d 4h'],
    [2 * 24 * 3_600_000, '2d'],
    [59 * 60_000 + 59_999, '59m'],
    [20_000, '1m'],
    [0, ''],
    [-5_000, ''],
  ]
  for (const [ms, want] of cases) assert.equal(resetsIn(ms), want, `${ms} ms`)
})

test('warning and danger tones', () => {
  const tones = [100, 21, 20.4, 20, 11, 10.4, 10, 0].map((left) => usageRows(limits([{ name: 'seven_day', label: 'Weekly', left, resetsAt: 0 }]), now)[0].tone)
  assert.deepEqual(tones, ['plain', 'plain', 'warning', 'warning', 'warning', 'danger', 'danger', 'danger'])
})
