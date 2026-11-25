import assert from 'node:assert/strict'
import { test } from 'node:test'
import { pollsServers, turnEnded } from './serversRead.ts'

test('servers are polled only while their tab shows a running thread', () => {
  const cases: [boolean, Parameters<typeof pollsServers>[1], boolean][] = [
    [true, 'working', true],
    [true, 'idle', true],
    [true, 'waiting', true],
    [false, 'working', false],
    [true, 'settled', false],
    [true, undefined, false],
  ]
  for (const [shown, state, want] of cases) assert.equal(pollsServers(shown, state), want, `${shown} ${state}`)
})

test('a turn ends when a working thread stops working', () => {
  const cases: [Parameters<typeof turnEnded>[0], Parameters<typeof turnEnded>[1], boolean][] = [
    ['working', 'idle', true],
    ['working', 'waiting', true],
    ['working', 'settled', true],
    ['working', 'working', false],
    ['idle', 'working', false],
    ['idle', 'waiting', false],
    [undefined, 'idle', false],
    ['working', undefined, false],
  ]
  for (const [prev, next, want] of cases) assert.equal(turnEnded(prev, next), want, `${prev} -> ${next}`)
})
