import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { Limits } from './types.ts'
import { providerColor, resetsIn, usageRows } from './usage.ts'

const now = Date.UTC(2026, 10, 26, 12, 0)
const limits = (windows: Limits['windows']): Limits => ({ provider: 'claude', providerLabel: 'Claude', windows, at: now, unsupported: false, error: '' })

test('percent left from get_usage', () => {
  const rows = usageRows(limits([
    { name: 'five_hour', label: 'Session', left: 74.5, resetsAt: now + 3 * 3_600_000 },
    { name: 'seven_day', label: 'Weekly', left: 60, resetsAt: now + (4 * 24 + 11) * 3_600_000 },
    { name: 'model:Fable', label: 'Weekly · Fable', left: 100, resetsAt: now + (4 * 24 + 11) * 3_600_000 },
    { name: 'spend', label: 'Monthly spend', left: 70, resetsAt: 0 },
  ]), now)
  assert.deepEqual(rows, [
    { label: 'Session', left: 75, resetsIn: '3h' },
    { label: 'Weekly', left: 60, resetsIn: '4d 11h' },
    { label: 'Weekly · Fable', left: 100, resetsIn: '4d 11h' },
    { label: 'Monthly spend', left: 70, resetsIn: '' },
  ])
})

test('stream utilization is scaled', () => {
  // A turn's rate_limit event says 0.96 of the week is used; Go keeps it as 4 left, to a hundredth.
  const [row] = usageRows(limits([{ name: 'seven_day', label: 'Weekly', left: 4.0000000000000036, resetsAt: now + 45 * 60_000 }]), now)
  assert.equal(row.left, 4)
  assert.equal(row.resetsIn, '45m')
  const [over] = usageRows(limits([{ name: 'five_hour', label: 'Session', left: -3, resetsAt: now - 1 }]), now)
  assert.deepEqual(over, { label: 'Session', left: 0, resetsIn: '' })
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

test('provider colours', () => {
  assert.equal(providerColor('claude'), 'var(--provider-claude)')
  assert.equal(providerColor('other'), 'var(--provider-other)')
})
