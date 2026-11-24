import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readStored, writeStored } from './storage.ts'

// A storage that holds values up to a quota, as the browser's does.
function useStorage(quota: number) {
  const items = new Map<string, string>()
  const storage = {
    getItem: (key: string) => items.get(key) ?? null,
    setItem: (key: string, value: string) => {
      if (value.length > quota) throw new Error('QuotaExceededError')
      items.set(key, value)
    },
    removeItem: (key: string) => items.delete(key),
  }
  Object.defineProperty(globalThis, 'localStorage', { value: storage, configurable: true })
}

test('a value too big to store removes the older one', () => {
  useStorage(20)
  writeStored('draft', 'short')
  assert.equal(readStored('draft', ''), 'short')
  writeStored('draft', 'far too long for the quota')
  assert.equal(readStored('draft', 'none'), 'none')
})
