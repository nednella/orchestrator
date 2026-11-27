import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createChipStore, MISS_KEPT_MS } from './chipStore.ts'
import type { FileChip } from './types.ts'

const chip = (path: string, exists = true): FileChip => ({ path, exists, dir: false, size: 3, abs: `/w/${path}`, inside: true, rel: path })

function fixture(missing: string[] = []) {
  const calls: { thread: string; paths: string[] }[] = []
  let time = 1000
  const store = createChipStore(
    async (thread, paths) => {
      calls.push({ thread, paths })
      return paths.map((p) => chip(p, !missing.includes(p)))
    },
    () => time,
  )
  return { store, calls, tick: (ms: number) => (time += ms) }
}

const settle = () => new Promise((resolve) => setTimeout(resolve, 0))

test('paths asked for in one tick go in one call per thread', async () => {
  const { store, calls } = fixture()
  store.ask('t1', 'a.ts')
  store.ask('t1', 'b.ts')
  store.ask('t1', 'a.ts')
  store.ask('t2', 'c.ts')
  assert.equal(store.chipOf('t1', 'a.ts'), undefined, 'nothing is known before the app answers')
  await settle()
  assert.deepEqual(calls, [
    { thread: 't1', paths: ['a.ts', 'b.ts'] },
    { thread: 't2', paths: ['c.ts'] },
  ])
  assert.equal(store.chipOf('t1', 'b.ts')?.exists, true)
  assert.equal(store.chipOf('t2', 'a.ts'), undefined, 'a thread has its own answers')
})

test('a path the app found is never asked again', async () => {
  const { store, calls, tick } = fixture()
  store.ask('t1', 'a.ts')
  await settle()
  tick(10 * MISS_KEPT_MS)
  store.ask('t1', 'a.ts')
  await settle()
  assert.equal(calls.length, 1)
})

test('a path the app did not find is asked again only after a while', async () => {
  const { store, calls, tick } = fixture(['gone.ts'])
  store.ask('t1', 'gone.ts')
  await settle()
  assert.equal(store.chipOf('t1', 'gone.ts')?.exists, false)
  tick(MISS_KEPT_MS - 1)
  store.ask('t1', 'gone.ts')
  await settle()
  assert.equal(calls.length, 1)
  tick(2)
  store.ask('t1', 'gone.ts')
  await settle()
  assert.equal(calls.length, 2)
})

test('a path still being asked for is not asked twice', async () => {
  const { store, calls } = fixture()
  store.ask('t1', 'a.ts')
  await Promise.resolve()
  store.ask('t1', 'a.ts')
  await settle()
  assert.deepEqual(calls.map((c) => c.paths), [['a.ts']])
})

test('a failed call leaves the paths as missing and tells the listeners', async () => {
  let heard = 0
  const store = createChipStore(async () => {
    throw new Error('down')
  })
  store.subscribe(() => heard++)
  store.ask('t1', 'a.ts')
  await settle()
  assert.equal(store.chipOf('t1', 'a.ts')?.exists, false)
  assert.equal(heard, 1)
})

test('a listener that left hears nothing', async () => {
  const { store } = fixture()
  let heard = 0
  const stop = store.subscribe(() => heard++)
  stop()
  store.ask('t1', 'a.ts')
  await settle()
  assert.equal(heard, 0)
})
