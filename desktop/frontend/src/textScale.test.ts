import assert from 'node:assert/strict'
import { test } from 'node:test'
import { snapScale, zoomedScale } from './textScale.ts'

const walk = (from: number, change: -1 | 1, times: number) => {
  const seen = [from]
  for (let i = 0; i < times; i++) seen.push(zoomedScale(seen[seen.length - 1], change))
  return seen
}

test('each step scales the text by the square root of 1.2', () => {
  const up = walk(1, 1, 8)
  for (let i = 1; i < up.length; i++) assert.equal(Math.abs(up[i] / up[i - 1] - Math.sqrt(1.2)) < 0.001, true, `${up[i - 1]} to ${up[i]}`)
  assert.deepEqual(up, [1, 1.0954, 1.2, 1.3145, 1.44, 1.5774, 1.728, 1.8929, 2.0736])
  assert.deepEqual(walk(1, -1, 8), [1, 0.9129, 0.8333, 0.7607, 0.6944, 0.6339, 0.5787, 0.5283, 0.4823])
})

test('the ladder stops eight steps either side of 1', () => {
  assert.equal(zoomedScale(2.0736, 1), 2.0736)
  assert.equal(zoomedScale(0.4823, -1), 0.4823)
})

test('reset goes back to 1 from any step', () => {
  for (const scale of [0.4823, 0.9129, 1, 1.44, 2.0736]) assert.equal(zoomedScale(scale, 0), 1)
})

test('a scale on the ladder stays as it is', () => {
  for (const scale of [...walk(1, 1, 8), ...walk(1, -1, 8)]) assert.equal(snapScale(scale), scale)
})

test('a stored scale off the ladder snaps to the nearest step', () => {
  const old = [0.85, 0.92, 1.1, 1.2, 1.35, 1.5].map(snapScale)
  assert.deepEqual(old, [0.8333, 0.9129, 1.0954, 1.2, 1.3145, 1.44])
  assert.equal(snapScale(0.3), 0.4823)
  assert.equal(snapScale(3), 2.0736)
})

test('a scale that is not a positive number snaps to 1', () => {
  for (const scale of [NaN, Infinity, -Infinity, 0, -1]) assert.equal(snapScale(scale), 1, String(scale))
  assert.equal(zoomedScale(NaN, 1), 1.0954)
})

test('zooming from a scale off the ladder moves from its nearest step', () => {
  assert.equal(zoomedScale(1.35, 1), 1.44)
  assert.equal(zoomedScale(0.85, -1), 0.7607)
})
