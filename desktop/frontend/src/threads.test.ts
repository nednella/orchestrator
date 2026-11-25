import assert from 'node:assert/strict'
import { test } from 'node:test'
import { currentTask, goneThreads, shortAgo, sortThreads } from './threads.ts'
import type { PR, Session, State } from './types.ts'

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
    hosted: true,
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

function pullRequest(state: PR['state'], checks: PR['checks']): PR {
  return { number: 91, url: '', state, checks, comments: 0, updatedAt: 0, conflicting: false, title: '', review: '', failedChecks: [], runningChecks: 0, passedChecks: 0, newComments: 0 }
}

test('a thread with a PR shows the PR, toned by its checks', () => {
  const tone = (state: PR['state'], checks: PR['checks']) => currentTask({ ...thread('t', 'idle'), issue: 444, pr: pullRequest(state, checks) })
  assert.deepEqual(tone('open', 'pending'), { kind: 'pr', number: 91, tone: 'pending' })
  assert.deepEqual(tone('draft', 'none'), { kind: 'pr', number: 91, tone: 'pending' })
  assert.deepEqual(tone('open', 'passing'), { kind: 'pr', number: 91, tone: 'passing' })
  assert.deepEqual(tone('open', 'failing'), { kind: 'pr', number: 91, tone: 'failing' })
  assert.deepEqual(tone('merged', 'passing'), { kind: 'pr', number: 91, tone: 'merged' })
  assert.deepEqual(tone('closed', 'failing'), { kind: 'pr', number: 91, tone: 'closed' })
})

test('a thread with an issue and no PR shows the issue', () => {
  assert.deepEqual(currentTask({ ...thread('t', 'working'), issue: 389 }), { kind: 'issue', number: 389 })
})

test('a thread with no issue and no PR shows no task', () => {
  assert.equal(currentTask(thread('t', 'idle')), null)
})

test('the time since the last event is compact', () => {
  const now = 100 * 24 * 3_600_000
  const minute = 60_000
  assert.equal(shortAgo(0, now), '')
  assert.equal(shortAgo(now - 59_000, now), 'now')
  assert.equal(shortAgo(now + 5_000, now), 'now')
  assert.equal(shortAgo(now - 2 * minute, now), '2m')
  assert.equal(shortAgo(now - 59 * minute, now), '59m')
  assert.equal(shortAgo(now - 60 * minute, now), '1h')
  assert.equal(shortAgo(now - (4 * 60 + 30) * minute, now), '4h')
  assert.equal(shortAgo(now - 24 * 60 * minute, now), '1d')
  assert.equal(shortAgo(now - 6 * 24 * 60 * minute, now), '6d')
  assert.equal(shortAgo(now - 7 * 24 * 60 * minute, now), '1w')
  assert.equal(shortAgo(now - 23 * 24 * 60 * minute, now), '3w')
})

test('a thread is gone when its project\'s new list leaves it out', () => {
  const other = { ...thread('other', 'idle'), project: 'blog' }
  assert.deepEqual(goneThreads([thread('a', 'idle'), thread('b', 'ended'), other], 'shop', [thread('a', 'idle')]), ['b'])
  assert.deepEqual(goneThreads([thread('a', 'idle')], 'shop', [thread('a', 'working'), thread('c', 'idle')]), [])
})
