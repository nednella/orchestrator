import assert from 'node:assert/strict'
import { test } from 'node:test'
import { middleTruncate, threadCardLines } from './threadCard.ts'
import type { PR, Thread } from './types.ts'

const thread = (over: Partial<Thread> = {}): Thread => ({
  id: 'shop/abcd2345', project: 'shop', n: 7, title: 'Flaky test', state: 'working', detail: '', lastEventAt: 0, workingSince: 0, createdAt: 0,
  issue: 0, model: '', effort: '', fast: false, branch: '', worktree: '', pr: null, git: null, prAttention: '', cleanup: '', cleanupReason: '',
  browser: false, evidence: 0, access: 'supervised', action: '', forkOf: '', forkAt: '', parent: '', side: false, tokens: 0, cost: 0, memory: 0, subagents: 0, start: '', startError: '', firstMessage: '', startQueue: [], setup: '', setupExit: 0, setupShell: '', setupProblem: '', ...over,
})

const pr: PR = {
  number: 618, url: '', state: 'draft', checks: 'passing', comments: 0, updatedAt: 0, conflicting: false, title: 'Keep the return path', review: '',
  failedChecks: [], runningChecks: 0, passedChecks: 0, newComments: 0,
}

const keys = (s: Thread, machine = '', title = '') => threadCardLines(s, machine, title).map((l) => l.key)

test('no PR line without a PR', () => {
  assert.deepEqual(keys(thread({ branch: 'issue-1' })), ['branch'])
  assert.deepEqual(keys(thread({ pr })), ['pr'])
})

test('the branch is truncated in the middle', () => {
  assert.equal(middleTruncate('issue-1', 32), 'issue-1')
  const cut = middleTruncate('experiment/websocket-retry-banner-with-backoff', 20)
  assert.equal(Array.from(cut).length, 20)
  assert.equal(cut.startsWith('experiment'), true)
  assert.equal(cut.endsWith('backoff'), true)
  assert.equal(cut.includes('…'), true)
  const line = threadCardLines(thread({ branch: 'experiment/websocket-retry-banner-with-backoff' }), '', '')[0]
  assert.equal(line.text.includes('…') && line.text.length <= 32, true)
})

test('middleTruncate never returns more than max characters', () => {
  assert.equal(middleTruncate('abcdef', 6), 'abcdef')
  for (const max of [0, 1, 2, 3, 5]) assert.equal(Array.from(middleTruncate('abcdefgh', max)).length <= max, true)
  assert.equal(middleTruncate('abcdefgh', 1), '…')
  assert.equal(middleTruncate('abcdefgh', 2), 'a…')
})

test('the issue title comes from the list', () => {
  assert.equal(threadCardLines(thread({ issue: 454 }), '', 'Split the page')[0].text, '#454 Split the page')
  assert.equal(threadCardLines(thread({ issue: 454 }), '', '')[0].text, '#454')
})

test('lines keep their order', () => {
  const all = thread({ branch: 'issue-454', pr, issue: 454, model: 'sonnet', effort: 'high' })
  assert.deepEqual(keys(all, 'Mac', 'Split'), ['machine', 'branch', 'pr', 'issue', 'model'])
  const line = threadCardLines(all, 'Mac', 'Split')[2]
  assert.equal(line.text, '#618 Keep the return path')
  assert.equal(line.detail, 'Draft')
  assert.deepEqual(keys(thread(), 'Mac'), ['machine'])
})

test('the machine line carries the thread\'s memory out of the machine\'s', () => {
  const GB = 2 ** 30
  const line = threadCardLines(thread({ memory: 0.2 * GB }), 'Mac', '', 36 * GB)[0]
  assert.deepEqual([line.key, line.text, line.detail], ['machine', 'Mac', '0.2 GB of 36 GB memory'])
  assert.equal(threadCardLines(thread({ memory: 0.2 * GB }), '', '')[0].text, '0.2 GB memory', 'no machine name: the memory alone')
  assert.equal(threadCardLines(thread(), 'Mac', '', 36 * GB)[0].detail, undefined, 'no memory sampled: the name alone')
})
