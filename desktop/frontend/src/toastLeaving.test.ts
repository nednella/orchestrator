import assert from 'node:assert/strict'
import { test } from 'node:test'
import { withLeaving, withoutLeaving } from './toastLeaving.ts'
import type { Toast } from './toastPile.ts'

const toast = (key: number): Toast => ({ key, tone: 'info', text: `t${key}` })
const keys = (list: { key: number; leaving?: boolean }[]) => list.map((t) => `${t.key}${t.leaving ? '*' : ''}`)

test('a toast that goes stays in its place, marked as leaving', () => {
  assert.deepEqual(keys(withLeaving([toast(1), toast(2), toast(3)], [toast(1), toast(3)])), ['1', '2*', '3'])
  assert.deepEqual(keys(withLeaving([toast(1), toast(2)], [toast(1)])), ['1', '2*'], 'the front one')
  assert.deepEqual(keys(withLeaving([toast(1), toast(2)], [toast(2)])), ['1*', '2'], 'the oldest one')
})

test('a new toast goes in front of those that leave', () => {
  assert.deepEqual(keys(withLeaving([toast(1), toast(2)], [toast(1), toast(3)])), ['1', '2*', '3'])
})

test('a toast that left and came back is no longer leaving', () => {
  const shown = withLeaving([toast(1), toast(2)], [toast(1)])
  assert.deepEqual(keys(withLeaving(shown, [toast(1), toast(2)])), ['1', '2'])
})

test('a leaving toast stays while the pile changes, and goes when it is over', () => {
  const shown = withLeaving([toast(1), toast(2)], [toast(1)])
  assert.deepEqual(keys(withLeaving(shown, [toast(1), toast(3)])), ['1', '2*', '3'])
  assert.deepEqual(keys(withoutLeaving(shown)), ['1'])
})

test('an empty pile leaves every toast', () => {
  assert.deepEqual(keys(withLeaving([toast(1), toast(2)], [])), ['1*', '2*'])
})
