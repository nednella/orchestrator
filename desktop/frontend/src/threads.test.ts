import assert from 'node:assert/strict'
import { test } from 'node:test'
import { STATE_LABEL } from './stateMeta.ts'
import { cleanupNote, threadGoal, descendantsOf, firstThread, goneThreads, isTopLevel, nextThread, shortAgo, sidebarThreads, sortThreads, splitSideChats, threadTreeOf, urgentThreads, waitingByProject } from './threads.ts'
import type { Project } from './types.ts'
import type { PR, Thread, State } from './types.ts'

function thread(id: string, state: State, lastEventAt = 0, parent = ''): Thread {
  return {
    id,
    project: 'shop',
    n: 1,
    title: id,
    state,
    detail: '',
    lastEventAt,
    createdAt: 0,
    wokeAt: 0,
    issue: 0,
    model: '',
    effort: '',
    fast: false,
    branch: '',
    worktree: '',
    pr: null,
    git: null,
    prAttention: '',
    cleanup: '',
    cleanupReason: '',
    browser: false,
    evidence: 0,
    access: 'supervised',
    action: '',
    forkOf: '',
    forkAt: '',
    parent,
    side: false,
    tokens: 0,
    cost: 0,
    memory: 0,
    subagents: 0,
  }
}

test('threads sort by state: attention, working, idle, then settled', () => {
  const sorted = sortThreads([thread('idle', 'idle'), thread('settled', 'settled'), thread('working', 'working'), thread('waiting', 'waiting')])
  assert.deepEqual(
    sorted.map((s) => s.id),
    ['waiting', 'working', 'idle', 'settled'],
  )
})

test('threads keep their order inside a live state', () => {
  const sorted = sortThreads([thread('b', 'working', 1), thread('a', 'working', 2), thread('c', 'idle'), thread('d', 'working', 3)])
  assert.deepEqual(
    sorted.map((s) => s.id),
    ['b', 'a', 'd', 'c'],
  )
})

test('settled threads go last, newest first', () => {
  const sorted = sortThreads([thread('old', 'settled', 1), thread('live', 'idle', 0), thread('new', 'settled', 3), thread('mid', 'settled', 2)])
  assert.deepEqual(
    sorted.map((s) => s.id),
    ['live', 'new', 'mid', 'old'],
  )
})

test('STATE_LABEL names attention', () => {
  assert.equal(STATE_LABEL.waiting, 'Attention')
  assert.equal(STATE_LABEL.working, 'Working')
  assert.equal(STATE_LABEL.idle, 'Idle')
  assert.equal(STATE_LABEL.settled, 'Sleeping')
})

function pullRequest(state: PR['state'], checks: PR['checks']): PR {
  return { number: 91, url: '', state, checks, comments: 0, updatedAt: 0, conflicting: false, title: '', review: '', failedChecks: [], runningChecks: 0, passedChecks: 0, newComments: 0 }
}

test('a thread with a PR shows the PR, toned by its checks', () => {
  const tone = (state: PR['state'], checks: PR['checks']) => threadGoal({ ...thread('t', 'idle'), issue: 444, pr: pullRequest(state, checks) })
  assert.deepEqual(tone('open', 'pending'), { kind: 'pr', number: 91, tone: 'pending' })
  assert.deepEqual(tone('draft', 'none'), { kind: 'pr', number: 91, tone: 'pending' })
  assert.deepEqual(tone('open', 'passing'), { kind: 'pr', number: 91, tone: 'passing' })
  assert.deepEqual(tone('open', 'failing'), { kind: 'pr', number: 91, tone: 'failing' })
  assert.deepEqual(tone('merged', 'passing'), { kind: 'pr', number: 91, tone: 'merged' })
  assert.deepEqual(tone('closed', 'failing'), { kind: 'pr', number: 91, tone: 'closed' })
})

test('a thread with an issue and no PR shows the issue', () => {
  assert.deepEqual(threadGoal({ ...thread('t', 'working'), issue: 389 }), { kind: 'issue', number: 389 })
})

test('a thread with no issue and no PR shows no goal', () => {
  assert.equal(threadGoal(thread('t', 'idle')), null)
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
  assert.deepEqual(goneThreads([thread('a', 'idle'), thread('b', 'settled'), other], 'shop', [thread('a', 'idle')]), ['b'])
  assert.deepEqual(goneThreads([thread('a', 'idle')], 'shop', [thread('a', 'working'), thread('c', 'idle')]), [])
})

test('children are not in the sidebar', () => {
  const threads = [thread('lead', 'working'), thread('child', 'working', 0, 'lead'), thread('grandchild', 'idle', 0, 'child')]
  assert.deepEqual(
    sidebarThreads(threads, 'shop').map((s) => s.id),
    ['lead'],
  )
  const byId = new Map(threads.map((s) => [s.id, s]))
  assert.equal(isTopLevel(threads[0], byId), true)
  assert.equal(isTopLevel(threads[1], byId), false)
})

test('the sidebar selects the root of an open child', () => {
  const threads = [thread('lead', 'working'), thread('child', 'working', 0, 'lead'), thread('grandchild', 'idle', 0, 'child'), thread('other', 'idle')]
  const top = threadTreeOf(threads).top
  assert.equal(top.get('grandchild')?.id, 'lead')
  assert.equal(top.get('child')?.id, 'lead')
  assert.equal(top.get('other')?.id, 'other')
})

test('a lost child shows', () => {
  const threads = [thread('lead', 'idle'), thread('orphan', 'working', 0, 'deleted-lead')]
  assert.equal(isTopLevel(threads[1], new Map(threads.map((s) => [s.id, s]))), true)
  assert.deepEqual(
    sortThreads(threads).map((s) => s.id),
    ['orphan', 'lead'],
  )
})

test('a waiting grandchild puts its top thread in attention', () => {
  const threads = [thread('busy', 'working'), thread('lead', 'idle'), thread('child', 'working', 0, 'lead'), thread('grandchild', 'waiting', 0, 'child')]
  assert.deepEqual(
    sortThreads(threads).map((s) => s.id),
    ['lead', 'busy'],
  )
  assert.equal(threadTreeOf(threads).attention.get('lead')?.id, 'grandchild')
  assert.deepEqual(
    descendantsOf('lead', threads).map((s) => s.id),
    ['child', 'grandchild'],
  )
})

test('a thread that waits needs the owner before its children', () => {
  const threads = [thread('lead', 'waiting'), thread('child', 'waiting', 0, 'lead')]
  assert.equal(threadTreeOf(threads).attention.get('lead')?.id, 'lead')
  assert.equal(threadTreeOf([thread('alone', 'idle')]).attention.get('alone'), undefined)
})

test('a thread needs the owner for its nearest waiting descendant', () => {
  const threads = [thread('lead', 'working'), thread('a', 'working', 0, 'lead'), thread('deep', 'waiting', 0, 'a'), thread('b', 'waiting', 0, 'lead')]
  assert.equal(threadTreeOf(threads).attention.get('lead')?.id, 'b')
})

test('rail counts a child in its parent\'s project', () => {
  const threads = [thread('lead', 'working'), { ...thread('child', 'waiting', 0, 'lead'), project: 'blog' }, { ...thread('own', 'waiting'), project: 'blog' }]
  assert.deepEqual(waitingByProject(threads), new Map([['shop', 1], ['blog', 1]]))
  assert.deepEqual(
    sidebarThreads(threads, 'blog').map((s) => s.id),
    ['own'],
  )
})

test('the next thread that needs the owner is the top thread of a waiting child', () => {
  const projects = [{ key: 'shop' }, { key: 'blog' }] as Project[]
  const threads = [thread('replied', 'idle'), thread('lead', 'working'), thread('child', 'waiting', 0, 'lead'), { ...thread('asks', 'waiting'), project: 'blog' }, thread('asleep', 'settled')]
  assert.deepEqual(
    urgentThreads(projects, threads).map((s) => s.id),
    ['lead', 'asks', 'replied'],
  )
})

test('the first thread opened is top-level, a lost child too', () => {
  const threads = [thread('child', 'working', 0, 'lead'), thread('lead', 'idle')]
  assert.equal(firstThread(threads, threadTreeOf(threads))?.id, 'lead')
  const lost = [thread('orphan', 'working', 0, 'deleted-lead'), thread('lead', 'idle')]
  assert.equal(firstThread(lost, threadTreeOf(lost))?.id, 'orphan')
})

test('the thread opened after a sleep is the next live top-level thread in sidebar order', () => {
  const threads = [thread('child', 'working', 0, 'lead'), thread('asleep', 'settled'), thread('lead', 'idle'), thread('busy', 'working'), thread('leaving', 'working')]
  assert.equal(nextThread(threads, 'shop', 'leaving')?.id, 'busy')
  assert.equal(nextThread([thread('leaving', 'idle'), thread('child', 'idle', 0, 'leaving')], 'shop', 'leaving'), undefined)
})

test('a kept clean-up says kept and why', () => {
  const s = { ...thread('a', 'settled'), cleanup: 'blocked' as const, cleanupReason: 'a shell is open in this worktree' }
  assert.equal(cleanupNote(s), 'Kept: a shell is open in this worktree')
})

test('side chats are kept apart from the threads', () => {
  const side = { ...thread('shop/side2345', 'working'), forkOf: 'shop/a', side: true }
  const rows = [thread('shop/a', 'idle'), side, thread('shop/b', 'waiting')]
  const { threads, sideChats } = splitSideChats(rows)
  assert.deepEqual(threads.map((s) => s.id), ['shop/a', 'shop/b'])
  assert.deepEqual(sideChats, [side])
})
