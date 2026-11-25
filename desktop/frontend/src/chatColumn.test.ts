import assert from 'node:assert/strict'
import { test } from 'node:test'
import { COLUMN_DEFAULT_REM, COLUMN_KEY, clampColumn, dragColumn, storedColumn } from './chatColumn.ts'
import { readStored, writeStored } from './storage.ts'

function useStorage() {
  const items = new Map<string, string>()
  const storage = {
    getItem: (key: string) => items.get(key) ?? null,
    setItem: (key: string, value: string) => items.set(key, value),
    removeItem: (key: string) => items.delete(key),
  }
  Object.defineProperty(globalThis, 'localStorage', { value: storage, configurable: true })
}

test('clampColumn keeps the width between the default and the pane', () => {
  assert.equal(clampColumn(60, 80), 60)
  assert.equal(clampColumn(30, 80), COLUMN_DEFAULT_REM)
  assert.equal(clampColumn(120, 80), 80)
})

test('clampColumn gives the default in a pane narrower than it', () => {
  assert.equal(clampColumn(60, 40), COLUMN_DEFAULT_REM)
})

test('dragColumn gives a little under the default, and none past the pane', () => {
  assert.equal(dragColumn(60, 80), 60)
  assert.equal(dragColumn(120, 80), 80)
  assert.equal(dragColumn(COLUMN_DEFAULT_REM - 5, 80), COLUMN_DEFAULT_REM - 1)
  assert.equal(dragColumn(0, 80), COLUMN_DEFAULT_REM - 2, 'the give stops at 2rem')
})

test('a saved width is read back, and anything else is the default', () => {
  useStorage()
  assert.equal(storedColumn(readStored(COLUMN_KEY, null)), COLUMN_DEFAULT_REM, 'nothing saved yet')
  writeStored(COLUMN_KEY, 62.5)
  assert.equal(storedColumn(readStored(COLUMN_KEY, null)), 62.5)
  for (const value of ['62', 20, Number.NaN, null, {}]) assert.equal(storedColumn(value), COLUMN_DEFAULT_REM, String(value))
})
