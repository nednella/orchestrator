import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { AgentItem } from './chatTimeline.ts'
import { cardPlacement, cardSections, lineageLabel } from './contextCard.ts'
import type { CardFacts } from './contextCard.ts'
import type { PR, Session } from './types.ts'

const session = (over: Partial<Session> = {}): Session => ({
  id: 'shop/abcd2345',
  project: 'shop',
  n: 7,
  title: 'Flaky test',
  state: 'working',
  detail: '',
  lastEventAt: 0,
  createdAt: 0,
  wokeAt: 0,
  issue: 0,
  model: '',
  effort: '',
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
  access: 'supervised',
  hosted: true,
  action: '',
  forkOf: '',
  parent: '',
  tokens: 0,
  cost: 0,
  memory: 0,
  subagents: 0,
  ...over,
})

const facts = (over: Partial<CardFacts> = {}): CardFacts => ({ machine: '', folder: '', editor: '', issueTitle: '', servers: [], agents: [], children: [], parent: null, forks: [], ...over })

const pr: PR = {
  number: 618,
  url: 'https://github.com/acme/shop/pull/618',
  state: 'draft',
  checks: 'passing',
  comments: 0,
  updatedAt: 0,
  conflicting: false,
  title: 'Keep the return path',
  review: '',
  failedChecks: [],
  runningChecks: 0,
  passedChecks: 4,
  newComments: 0,
}

const agent = (id: string, state: AgentItem['state']): AgentItem => ({ kind: 'agent', key: `agent-${id}`, id, turn: 't1', description: id, type: 'general-purpose', startedAt: 0, endedAt: null, state, tools: 0, steps: [], result: null })

const kinds = (s: Session, f: CardFacts) => cardSections(s, f).map((section) => section.rows.map((row) => row.kind))

test('no row without data', () => {
  assert.deepEqual(cardSections(session(), facts()), [])
  assert.deepEqual(kinds(session({ branch: 'issue-612', issue: 612 }), facts({ machine: 'MacBook Air', folder: '/code/shop' })), [['workspace'], ['branch', 'issue']])
  assert.deepEqual(kinds(session(), facts({ editor: 'VS Code' })), [['editor']], 'an editor alone is a row')
})

test('a thread with no PR has no PR row', () => {
  const rows = cardSections(session({ branch: 'issue-612' }), facts()).flatMap((section) => section.rows)
  assert.equal(
    rows.some((row) => row.kind === 'pr'),
    false,
  )
  assert.deepEqual(kinds(session({ branch: 'issue-612', pr }), facts()), [['branch', 'pr']])
})

test('an empty section is dropped', () => {
  const keys = cardSections(session({ branch: 'issue-612' }), facts({ machine: 'MacBook Air', folder: '/code/shop', editor: 'VS Code' })).map((section) => section.key)
  assert.deepEqual(keys, ['workspace', 'git'], 'no servers and no lineage: no heading for either')
  const served = cardSections(session(), facts({ servers: [{ port: 5173, pid: 41, command: 'vite --port 5173' }] }))
  assert.deepEqual(
    served.map((section) => [section.key, section.heading]),
    [['servers', 'Servers']],
  )
})

test('lineage counts the running', () => {
  const lead = session({ id: 'shop/lead2345', title: 'Lead' })
  assert.equal(lineageLabel(facts({ parent: lead })), 'Lineage')
  const children = [session({ id: 'shop/kid22345', state: 'working' }), session({ id: 'shop/kid32345', state: 'idle' })]
  const agents = [agent('a1', 'running'), agent('a2', 'done')]
  const f = facts({ parent: lead, children, agents, forks: [session({ id: 'shop/fork2345', state: 'working' })] })
  assert.equal(lineageLabel(f), 'Lineage · 2 running', 'a working child and a running subagent; a fork is not a worker of this thread')
  const lineage = cardSections(session(), f).find((section) => section.key === 'lineage')
  assert.equal(lineage?.heading, 'Lineage · 2 running')
  assert.deepEqual(
    lineage?.rows.map((row) => row.key),
    ['parent-shop/lead2345', 'child-shop/kid22345', 'child-shop/kid32345', 'agent-a1', 'agent-a2', 'fork-shop/fork2345'],
  )
})

test('popover when the chat would keep under 640 px', () => {
  // 640 + 280 + 12 + 32 + 2 × 16 = 996
  assert.equal(cardPlacement(996, 600), 'inline')
  assert.equal(cardPlacement(995, 600), 'popover')
})

test('popover when shorter than 160 px', () => {
  assert.equal(cardPlacement(1400, 160), 'inline')
  assert.equal(cardPlacement(1400, 159), 'popover')
})
