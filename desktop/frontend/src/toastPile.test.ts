import assert from 'node:assert/strict'
import { test } from 'node:test'
import { addToast, MAX_TOASTS } from './toastPile.ts'
import type { Toast, ToastInput } from './toastPile.ts'

const info = (text: string, more: Partial<ToastInput> = {}): ToastInput => ({ tone: 'info', text, ...more })

function pile(...inputs: ToastInput[]): Toast[] {
  return inputs.reduce<Toast[]>((list, input, i) => addToast(list, input, i + 1).list, [])
}

const texts = (list: Toast[]) => list.map((t) => t.text)

test('the newest toast is at the end, which is the front of the pile', () => {
  assert.deepEqual(texts(pile(info('a'), info('b'), info('c'))), ['a', 'b', 'c'])
})

test('the pile keeps four toasts and drops the oldest first', () => {
  const list = pile(info('a'), info('b'), info('c'), info('d'), info('e'))
  assert.equal(MAX_TOASTS, 4)
  assert.deepEqual(texts(list), ['b', 'c', 'd', 'e'])
})

test('a sticky toast outlives older toasts that are not sticky', () => {
  const list = pile(info('wait', { sticky: true }), info('a'), info('b'), info('c'), info('d'))
  assert.deepEqual(texts(list), ['wait', 'b', 'c', 'd'])
})

test('when every toast is sticky the oldest goes, never the new one', () => {
  const sticky = (text: string) => info(text, { sticky: true })
  const list = pile(sticky('a'), sticky('b'), sticky('c'), sticky('d'), sticky('e'))
  assert.deepEqual(texts(list), ['b', 'c', 'd', 'e'])
})

test('a repeated message replaces itself in front, under its first key', () => {
  const first = addToast([], info('Copied'), 1)
  const other = addToast(first.list, info('Fast mode on'), 2)
  const again = addToast(other.list, info('Copied'), 3)
  assert.equal(again.key, 1)
  assert.deepEqual(texts(again.list), ['Fast mode on', 'Copied'])
})

test('a toast with the id of one shown replaces it, with its new text', () => {
  const first = addToast([], info('Retry', { id: 'network', sticky: true }), 1)
  const again = addToast(first.list, info('Retrying', { id: 'network', sticky: true, busy: true }), 2)
  assert.equal(again.key, 1)
  assert.deepEqual(again.list.map((t) => [t.text, t.busy]), [['Retrying', true]])
})

test('the same text with another detail is another toast', () => {
  assert.equal(pile(info('Copied', { detail: 'a' }), info('Copied', { detail: 'b' })).length, 2)
})

test('a new toast covers a needs attention toast, which stays in the pile', () => {
  const waiting: ToastInput = { tone: 'waiting', text: 'Fix needs attention', sticky: true }
  const list = pile(waiting, info('Copied'))
  assert.deepEqual(texts(list), ['Fix needs attention', 'Copied'])
  assert.equal(list[0]?.sticky, true)
})
