import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { AgentItem } from './chatTimeline.ts'
import { cardLineage, cardPlacement, cardSections, changesTip, editorLabel, lineageLabel, serverMeta, VS_CODE_APP } from './contextCard.ts'
import type { CardFacts } from './contextCard.ts'
import type { PR, Thread } from './types.ts'

const thread = (over: Partial<Thread> = {}): Thread => ({
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
  access: 'supervised',
  action: '',
  forkOf: '',
  forkAt: '',
  parent: '',
  side: false,
  tokens: 0,
  cost: 0,
  memory: 0,
  subagents: 0,
  ...over,
})

const facts = (over: Partial<CardFacts> = {}): CardFacts => ({ machine: '', folder: '', editor: '', issueTitle: '', changes: null, servers: [], agents: [], children: [], parent: null, forks: [], sideChats: [], ...over })

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

const agent = (id: string, state: AgentItem['state']): AgentItem => ({ kind: 'agent', key: `agent-${id}`, id, turn: 't1', description: id, type: 'general-purpose', prompt: '', model: '', startedAt: 0, endedAt: null, state, tools: 0, steps: [], result: null })

const kinds = (s: Thread, f: CardFacts) => cardSections(s, f).map((section) => section.rows.map((row) => row.kind))

test('no row without data', () => {
  assert.deepEqual(cardSections(thread(), facts()), [])
  assert.deepEqual(kinds(thread({ branch: 'issue-612', issue: 612 }), facts({ machine: 'MacBook Air', folder: '/code/shop' })), [['workspace', 'editor'], ['branch', 'issue']])
  assert.deepEqual(kinds(thread(), facts({ editor: 'Zed' })), [], 'no folder to open: no editor row')
})

test('a thread with no PR has no PR row', () => {
  const rows = cardSections(thread({ branch: 'issue-612' }), facts()).flatMap((section) => section.rows)
  assert.equal(
    rows.some((row) => row.kind === 'pr'),
    false,
  )
  assert.deepEqual(kinds(thread({ branch: 'issue-612', pr }), facts()), [['branch', 'pr']])
})

test('the changes row sits under the branch and shows only with a base', () => {
  const t = thread({ branch: 'issue-612', pr })
  assert.deepEqual(kinds(t, facts({ changes: { base: 'origin/main', added: 12, removed: 3 } })), [['branch', 'changes', 'pr']])
  assert.deepEqual(kinds(t, facts({ changes: { base: '', added: 0, removed: 0 } })), [['branch', 'pr']])
  assert.deepEqual(kinds(t, facts()), [['branch', 'pr']])
  assert.equal(changesTip('origin/main'), 'Lines this branch changes against origin/main, committed or not. New files count once they are added.')
})

test('an empty section is dropped', () => {
  const keys = cardSections(thread({ branch: 'issue-612' }), facts({ machine: 'MacBook Air', folder: '/code/shop', editor: 'VS Code' })).map((section) => section.key)
  assert.deepEqual(keys, ['workspace', 'git'], 'no servers and no lineage: no heading for either')
  const served = cardSections(thread(), facts({ servers: [{ port: 5173, pid: 41, command: 'vite --port 5173', started: 0 }] }))
  assert.deepEqual(
    served.map((section) => [section.key, section.heading]),
    [['servers', 'Servers']],
  )
})

test('lineage counts the running', () => {
  const lead = thread({ id: 'shop/lead2345', title: 'Lead' })
  assert.equal(lineageLabel(facts({ parent: lead })), 'Lineage')
  const children = [thread({ id: 'shop/kid22345', state: 'working' }), thread({ id: 'shop/kid32345', state: 'idle' })]
  const agents = [agent('a1', 'running'), agent('a2', 'done')]
  const f = facts({ parent: lead, children, agents, forks: [thread({ id: 'shop/fork2345', state: 'working' })] })
  assert.equal(lineageLabel(f), 'Lineage · 2 running', 'a working child and a running subagent; a fork is not a worker of this thread')
  const lineage = cardSections(thread(), f).find((section) => section.key === 'lineage')
  assert.equal(lineage?.heading, 'Lineage · 2 running')
  assert.deepEqual(
    lineage?.rows.map((row) => row.key),
    ['parent-shop/lead2345', 'child-shop/kid22345', 'child-shop/kid32345', 'agent-a1', 'agent-a2', 'fork-shop/fork2345'],
  )
})

test("the thread's side chats are a list of their own, after the lineage", () => {
  const asks = thread({ id: 'shop/side2345', title: 'Why the retry?', state: 'waiting', forkOf: 'shop/abcd2345', side: true })
  const runs = thread({ id: 'shop/side3345', title: 'Side chat', state: 'working', forkOf: 'shop/abcd2345', side: true })
  const sections = cardSections(thread(), facts({ forks: [thread({ id: 'shop/fork2345' })], sideChats: [asks, runs] }))
  assert.deepEqual(
    sections.map((section) => [section.key, section.heading]),
    [['lineage', 'Lineage'], ['sideChats', 'Side chats · 1 running']],
  )
  assert.deepEqual(sections[1].rows, [{ kind: 'sideChat', key: 'side-shop/side2345', thread: asks }, { kind: 'sideChat', key: 'side-shop/side3345', thread: runs }])
  assert.equal(cardSections(thread(), facts({ sideChats: [asks] }))[0].heading, 'Side chats', 'none of them running')
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

test('the editor row names the editor only when it is known', () => {
  assert.equal(editorLabel(''), 'Open in editor')
  assert.equal(editorLabel('Visual Studio Code'), 'Open in VS Code')
  assert.equal(editorLabel('Zed'), 'Open in Zed')
  const row = cardSections(thread(), facts({ folder: '/code/shop' }))[0].rows.find((r) => r.kind === 'editor')
  assert.deepEqual(row, { kind: 'editor', key: 'editor', editor: '' })
})

test("the editor row opens the config's editor app", () => {
  const row = cardSections(thread(), facts({ folder: '/code/shop', editor: VS_CODE_APP }))[0].rows.find((r) => r.kind === 'editor')
  assert.deepEqual(row, { kind: 'editor', key: 'editor', editor: 'Visual Studio Code' })
  assert.equal(editorLabel(VS_CODE_APP), 'Open in VS Code')
})

test('a server says how long it has run, once known', () => {
  assert.equal(serverMeta('/usr/local/bin/vite --port 5173', '4m'), 'Agent · vite · 4m')
  assert.equal(serverMeta('node server/api.js', ''), 'Agent · node', 'no start time, no age')
})

test("a fork's parent is the thread it was forked from", () => {
  const source = thread({ id: 'shop/srce2345', title: 'Source' })
  const fork = thread({ id: 'shop/fork2345', forkOf: source.id })
  const other = thread({ id: 'shop/othr2345', forkOf: source.id })
  const all = [source, fork, other]
  assert.equal(cardLineage(fork, all).parent, source)
  assert.deepEqual(cardLineage(source, all), { parent: null, children: [], forks: [fork, other] })
})

test('a delegated thread names the thread that delegated it as its parent', () => {
  const lead = thread({ id: 'shop/lead2345' })
  const source = thread({ id: 'shop/srce2345' })
  const child = thread({ id: 'shop/kid22345', parent: lead.id, forkOf: source.id })
  const all = [lead, source, child]
  assert.equal(cardLineage(child, all).parent, lead, 'the delegation wins over the fork')
  assert.deepEqual(cardLineage(lead, all).children, [child])
  assert.equal(cardLineage(thread({ parent: 'shop/gone2345' }), all).parent, null, 'a parent gone from the list is no row')
})
