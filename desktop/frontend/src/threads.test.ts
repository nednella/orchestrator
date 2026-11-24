import assert from 'node:assert/strict'
import { test } from 'node:test'
import { sortThreads } from './threads.ts'
import type { Session, State } from './types.ts'

function thread(id: string, state: State, lastEventAt = 0): Session {
  return {
    id,
    project: 'shop',
    n: 1,
    title: id,
    state,
    detail: '',
    lastEventAt,
    createdAt: 0,
    beganAt: 0,
    issue: 0,
    model: '',
    effort: '',
    agent: 'claude',
    history: [],
    branch: '',
    worktree: '',
    pr: null,
    git: null,
    prAttention: '',
    cleanup: '',
    cleanupReason: '',
    browser: false,
    evidence: 0,
    chat: false,
    action: '',
    tokens: 0,
    cost: 0,
    resumable: false,
    memory: 0,
  }
}

test('threads sort by state: needs you, working, idle, then ended', () => {
  const sorted = sortThreads([thread('idle', 'idle'), thread('ended', 'ended'), thread('working', 'working'), thread('waiting', 'waiting')])
  assert.deepEqual(
    sorted.map((s) => s.id),
    ['waiting', 'working', 'idle', 'ended'],
  )
})

test('threads keep their order inside a live state', () => {
  const sorted = sortThreads([thread('b', 'working', 1), thread('a', 'working', 2), thread('c', 'idle'), thread('d', 'working', 3)])
  assert.deepEqual(
    sorted.map((s) => s.id),
    ['b', 'a', 'd', 'c'],
  )
})

test('ended threads come newest first', () => {
  const sorted = sortThreads([thread('old', 'ended', 1), thread('new', 'ended', 3), thread('mid', 'ended', 2)])
  assert.deepEqual(
    sorted.map((s) => s.id),
    ['new', 'mid', 'old'],
  )
})
