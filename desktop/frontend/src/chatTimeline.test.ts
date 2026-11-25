import assert from 'node:assert/strict'
import { test } from 'node:test'
import { EMPTY_CHAT, mergeEvents } from './chatEvents.ts'
import { buildTimeline } from './chatTimeline.ts'
import type { TimelineItem } from './chatTimeline.ts'
import type { ChatEvent } from './types.ts'

type Draft = Omit<ChatEvent, 'seq' | 'at'> & { at?: number }

// log numbers the drafts from 1 and, unless a draft says otherwise, puts each a second after the one before.
function log(...drafts: Draft[]): ChatEvent[] {
  return drafts.map((d, i) => ({ at: (i + 1) * 1000, ...d, seq: i + 1 }))
}

const user = (id: string, text: string, at?: number): Draft => ({ kind: 'user', id, text, ...(at ? { at } : {}) })
const message = (id: string, text: string): Draft => ({ kind: 'message', id, text })
const toolUse = (id: string, tool: string, input: unknown = {}): Draft => ({ kind: 'tool_use', id, tool, input })
const toolResult = (parent: string, text: string, error = false): Draft => ({ kind: 'tool_result', parent, text, ...(error ? { error } : {}) })
const approval = (id: string, tool: string, input: unknown = {}, parent?: string): Draft => ({ kind: 'approval', id, tool, input, ...(parent ? { parent } : {}) })
const answer = (id: string, text: 'allow' | 'deny'): Draft => ({ kind: 'answer', id, text })
const turnEnd = (error = false, at?: number): Draft => ({ kind: 'turn_end', cost: 0.01, ...(error ? { error } : {}), ...(at ? { at } : {}) })

const kinds = (items: TimelineItem[]) => items.map((i) => i.kind)
const only = <K extends TimelineItem['kind']>(items: TimelineItem[], kind: K) => items.filter((i): i is Extract<TimelineItem, { kind: K }> => i.kind === kind)

test('reply with no tools has no fold', () => {
  const t = buildTimeline(log(user('u1', 'Hi'), { kind: 'init', id: 's' }, message('msg_1', 'Hello.'), turnEnd()), {})
  assert.deepEqual(kinds(t.items), ['user', 'reply'])
  assert.equal(only(t.items, 'reply')[0].text, 'Hello.')
  assert.equal(t.turnOpen, false)
})

test('finished turn folds tools into one work row', () => {
  const events = log(
    user('u1', 'Fix it', 10_000),
    message('msg_1', "I'll read the router."),
    toolUse('t1', 'Read', { file_path: 'a.ts' }),
    toolResult('t1', 'export const a = 1'),
    message('msg_2', 'Now the edit.'),
    toolUse('t2', 'Edit', { file_path: 'a.ts' }),
    toolResult('t2', 'The file was updated.'),
    toolResult('t9', 'a result with no call'),
    message('msg_3', 'Done.'),
    turnEnd(false, 134_000),
  )
  const open = buildTimeline(events.slice(0, -1), {})
  assert.equal(open.turnOpen, true)
  assert.deepEqual(kinds(open.items), ['user', 'reply', 'work', 'reply', 'work', 'reply'], 'an open turn stays in order')
  assert.equal(only(open.items, 'work')[0].endedAt, null)

  const t = buildTimeline(events, {})
  assert.equal(t.turnOpen, false)
  assert.deepEqual(kinds(t.items), ['user', 'work', 'reply'])
  const [work] = only(t.items, 'work')
  assert.equal(work.endedAt! - work.startedAt, 134_000 - 10_000)
  assert.deepEqual(
    work.calls.map((c) => [c.id, c.tool, c.result, c.error]),
    [
      ['t1', 'Read', 'export const a = 1', false],
      ['t2', 'Edit', 'The file was updated.', false],
    ],
  )
  assert.deepEqual(
    work.steps.map((st) => (st.kind === 'call' ? st.call.id : st.text)),
    ["I'll read the router.", 't1', 'Now the edit.', 't2'],
    'notes sit between the calls in the order they came',
  )
  assert.equal(only(t.items, 'reply')[0].text, 'Done.')
  assert.equal(new Set(t.items.map((i) => i.key)).size, t.items.length, 'keys are unique')
})

test('blocks of one message join', () => {
  const t = buildTimeline(log(user('u1', 'Hi'), message('msg_1', 'First block.'), message('msg_1', 'Second block.'), turnEnd()), {})
  assert.deepEqual(kinds(t.items), ['user', 'reply'])
  assert.equal(only(t.items, 'reply')[0].text, 'First block.\n\nSecond block.')
})

test('deltas give way to the message', () => {
  const delta = (text: string): ChatEvent => ({ seq: 0, at: 0, kind: 'delta', id: 'msg_1', text })
  const [u, first, second] = log(user('u1', 'Hi'), message('msg_1', 'First block.'), message('msg_1', 'Second block.'))
  const reply = (s: typeof EMPTY_CHAT) => {
    const replies = only(buildTimeline(s.events, s.live).items, 'reply')
    assert.equal(replies.length, 1)
    return replies[0]
  }

  let s = mergeEvents(EMPTY_CHAT, [u, delta('First '), delta('block.')]).state
  assert.deepEqual([reply(s).text, reply(s).streaming], ['First block.', true])
  s = mergeEvents(s, [first, delta('Sec')]).state
  assert.deepEqual([reply(s).text, reply(s).streaming], ['First block.\n\nSec', true])
  s = mergeEvents(s, [delta('ond block.'), second]).state
  assert.deepEqual([reply(s).text, reply(s).streaming], ['First block.\n\nSecond block.', false])

  // Watch, then history: the history comes first, then what the stream sent meanwhile, the first block among it.
  let r = mergeEvents(EMPTY_CHAT, [u, first]).state
  r = mergeEvents(r, [delta('First '), delta('block.'), first, delta('Sec')]).state
  assert.equal(reply(r).text, 'First block.\n\nSec')
})

test('answered approval folds as a call answer', () => {
  const events = log(
    user('u1', 'Install it'),
    toolUse('t1', 'Bash', { command: 'npm install' }),
    approval('r1', 'Bash', { command: 'npm install' }),
    answer('r1', 'allow'),
    toolResult('t1', 'added 12 packages'),
    toolUse('t2', 'Bash', { command: 'rm -rf node_modules' }),
    approval('r2', 'Bash', { command: 'rm -rf node_modules' }),
    answer('r2', 'deny'),
    toolResult('t2', 'The user denied this tool.', true),
    message('msg_1', 'Installed.'),
    turnEnd(),
  )
  const t = buildTimeline(events, {})
  assert.deepEqual(kinds(t.items), ['user', 'work', 'reply'])
  assert.deepEqual(
    only(t.items, 'work')[0].calls.map((c) => [c.id, c.answer, c.error]),
    [
      ['t1', 'allow', false],
      ['t2', 'deny', true],
    ],
  )
  assert.equal(t.pending, null)
})

test('approvals pair with their calls by id', () => {
  const answers = (...drafts: Draft[]) => only(buildTimeline(log(user('u1', 'Clean up'), toolUse('t1', 'Bash'), toolUse('t2', 'Bash'), ...drafts, turnEnd()), {}).items, 'work')[0].calls.map((c) => [c.id, c.answer])
  assert.deepEqual(answers(approval('r2', 'Bash', {}, 't2'), answer('r2', 'deny'), approval('r1', 'Bash', {}, 't1'), answer('r1', 'allow')), [
    ['t1', 'allow'],
    ['t2', 'deny'],
  ])
  assert.deepEqual(answers(approval('r1', 'Bash'), answer('r1', 'allow'), approval('r2', 'Bash'), answer('r2', 'deny')), [
    ['t1', 'allow'],
    ['t2', 'deny'],
  ], 'an older log with no parent pairs in order')
})

test('unanswered approval expires at turn end', () => {
  const events = log(user('u1', 'Install it'), toolUse('t1', 'Bash', { command: 'npm install\nnpm test' }), approval('r1', 'Bash', { command: 'npm install\nnpm test' }))
  const waiting = buildTimeline(events, {}, 'shop/abcd2345')
  assert.deepEqual(waiting.pending, { id: 'shop/abcd2345', request: 'r1', tool: 'Bash', summary: 'npm install', kind: 'tool' })
  assert.deepEqual(kinds(waiting.items), ['user', 'work', 'approval'])
  assert.equal(only(waiting.items, 'approval')[0].expired, false)

  const ended = buildTimeline(log(...events, turnEnd(true)), {})
  assert.equal(ended.pending, null)
  assert.equal(ended.turnOpen, false)
  const [expired] = only(ended.items, 'approval')
  assert.deepEqual([expired.request, expired.tool, expired.expired], ['r1', 'Bash', true])
  assert.equal(only(ended.items, 'work')[0].calls[0].answer, undefined)
})

test('question and plan items', () => {
  const questions = { questions: [{ question: 'Which colour do you like?', header: 'Colour', options: [{ label: 'Red' }, { label: 'Blue' }], multiSelect: false }] }
  const plan = { plan: '# Move checkout\n\n1. Add the route' }
  const asking = log(user('u1', 'Ask me'), toolUse('t1', 'AskUserQuestion', questions), approval('r1', 'AskUserQuestion', questions))
  const t = buildTimeline(asking, {})
  assert.deepEqual(kinds(t.items), ['user', 'question'])
  const [q] = only(t.items, 'question')
  assert.deepEqual([q.id, q.request, q.input, q.answer, q.result, q.expired], ['t1', 'r1', questions, undefined, null, false])
  assert.equal(t.pending?.kind, 'question')
  assert.equal(t.pending?.summary, 'Which colour do you like?')

  const answered = buildTimeline(
    log(
      ...asking,
      answer('r1', 'allow'),
      toolResult('t1', 'User has answered your questions: "Which colour do you like?"="Red"'),
      toolUse('t2', 'ExitPlanMode', plan),
      approval('r2', 'ExitPlanMode', plan),
      answer('r2', 'allow'),
      toolResult('t2', 'User has approved your plan.'),
      toolUse('t3', 'Edit', { file_path: 'routes.tsx' }),
      toolResult('t3', 'The file was updated.'),
      message('msg_1', 'You picked red, and the route is in.'),
      turnEnd(),
    ),
    {},
  )
  assert.deepEqual(kinds(answered.items), ['user', 'work', 'question', 'plan', 'reply'], 'a question and a plan are never folded')
  assert.deepEqual(only(answered.items, 'work')[0].calls.map((c) => c.tool), ['Edit'])
  assert.equal(only(answered.items, 'question')[0].answer, 'allow')
  assert.equal(only(answered.items, 'question')[0].result?.includes('"Red"'), true)
  const [p] = only(answered.items, 'plan')
  assert.deepEqual([p.input, p.answer], [plan, 'allow'])
})

test('interrupt gives Stopped', () => {
  const notices = (...drafts: Draft[]) => only(buildTimeline(log(...drafts), {}).items, 'notice').map((n) => [n.text, n.error])
  assert.deepEqual(notices(user('u1', 'Go'), toolUse('t1', 'Bash'), { kind: 'interrupt' }, turnEnd(true)), [['Stopped', false]])
  assert.deepEqual(notices(user('u1', 'Go'), turnEnd(true)), [['The turn ended with an error', true]])
  assert.deepEqual(notices(user('u1', 'Go'), message('msg_1', 'On it.'), { kind: 'exit' }), [['Claude stopped', false]])
  assert.deepEqual(notices({ kind: 'interrupt' }, user('u1', 'Go'), turnEnd(true)), [['The turn ended with an error', true]], 'an interrupt while idle stops nothing')

  const exited = buildTimeline(log(user('u1', 'Go'), toolUse('t1', 'Bash'), { kind: 'exit', text: 'signal: killed', error: true }), {})
  assert.equal(exited.turnOpen, false)
  assert.deepEqual(kinds(exited.items), ['user', 'work', 'notice'])
  assert.equal(only(exited.items, 'notice')[0].error, true)
})

test('queued until delivered', () => {
  const sent = log(user('u1', 'Slow job'), message('msg_1', 'Working.'), { kind: 'queued', id: 'q1', text: 'Also run the tests', files: ['/m/a.png'] })
  const waiting = buildTimeline(sent, {})
  assert.deepEqual(waiting.queued, [{ id: 'q1', text: 'Also run the tests', files: ['/m/a.png'] }])
  assert.deepEqual(kinds(waiting.items), ['user', 'reply'])

  const delivered = buildTimeline(log(...sent, { kind: 'user', id: 'q1', text: 'Also run the tests', files: ['/m/a.png'] }), {})
  assert.deepEqual(delivered.queued, [])
  assert.deepEqual(only(delivered.items, 'user').map((u) => [u.id, u.files]), [
    ['u1', []],
    ['q1', ['/m/a.png']],
  ])
  assert.equal(delivered.turnOpen, true)
})

test('queued is not sent once the host drops it or Claude exits', () => {
  const sent = log(user('u1', 'Slow job'), { kind: 'queued', id: 'q1', text: '\nRun the tests\nand lint' }, { kind: 'queued', id: 'q2', text: 'Then lint' }, { kind: 'interrupt' })
  const held = buildTimeline(log(...sent, turnEnd(true)), {})
  assert.deepEqual(held.queued.map((q) => q.id), ['q1', 'q2'], 'Claude runs what it still holds after the interrupt')
  assert.deepEqual(only(held.items, 'notice').map((n) => n.text), ['Stopped'])

  const dropping = buildTimeline(log(...sent, { kind: 'unsent', id: 'q1' }), {})
  assert.deepEqual(dropping.queued.map((q) => q.id), ['q2'])
  assert.equal(dropping.items.at(-1)?.key, 'unsent-q1', 'the notice shows while the turn still runs')

  const dropped = buildTimeline(log(...sent, { kind: 'unsent', id: 'q1' }, turnEnd(true), user('q2', 'Then lint')), {})
  assert.deepEqual(dropped.queued, [])
  assert.deepEqual(only(dropped.items, 'notice').map((n) => n.text), ['Stopped', 'Not sent: Run the tests'])

  const exited = buildTimeline(log(user('u1', 'Slow job'), { kind: 'queued', id: 'q1', text: 'Run the tests' }, { kind: 'exit' }), {})
  assert.deepEqual(exited.queued, [])
  assert.deepEqual(only(exited.items, 'notice').map((n) => n.text), ['Claude stopped', 'Not sent: Run the tests'])
})

test('latest commands win', () => {
  const t = buildTimeline(
    log(
      { kind: 'commands', input: [{ name: 'old', description: 'Gone', argumentHint: '' }] },
      user('u1', 'Hi'),
      turnEnd(),
      { kind: 'commands', input: [{ name: 'greet', description: 'Say hi', argumentHint: '' }, { name: 'review', description: 'Review a PR', argumentHint: '<pr>', builtin: true }, { description: 'no name' }] },
    ),
    {},
  )
  assert.deepEqual(t.commands.map((c) => c.name), ['greet', 'review'])
  assert.equal(buildTimeline([], {}).commands.length, 0)
})
