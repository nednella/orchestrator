import assert from 'node:assert/strict'
import { test } from 'node:test'
import { childFacts } from './childCards.ts'
import type { Thread } from './types.ts'

const thread = (over: Partial<Thread>): Thread => ({
  id: 'shop/abcd2345',
  project: 'shop',
  n: 7,
  title: 'Flaky test',
  state: 'working',
  detail: '',
  lastEventAt: 0,
  createdAt: 0,
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
  access: 'read',
  action: '',
  forkOf: '',
  forkAt: '',
  parent: 'shop/lead2345',
  side: false,
  tokens: 0,
  cost: 0,
  memory: 0,
  subagents: 0,
  start: '',
  startError: '',
  firstMessage: '',
  startQueue: [],
  ...over,
})

test("facts follow the child's row", () => {
  assert.deepEqual(childFacts(thread({}), 'Asked title'), { title: 'Flaky test', project: 'shop', access: 'read', where: 'Folder', state: 'working' })
  assert.deepEqual(childFacts(thread({ title: 'Renamed', state: 'idle', access: 'full', branch: 'flaky-test', worktree: '/code/shop/trees/flaky-test', project: 'api' }), 'Asked title'), {
    title: 'Renamed',
    project: 'api',
    access: 'full',
    where: 'flaky-test',
    state: 'idle',
  })
  assert.equal(childFacts(thread({ branch: '', worktree: '/code/shop/trees/flaky-test' }), '').where, 'flaky-test', 'a worktree with no branch yet is named by its folder')
})

test('a gone child keeps its title', () => {
  assert.deepEqual(childFacts(undefined, 'Flaky test'), { title: 'Flaky test', project: '', access: '', where: '', state: 'gone' })
})
