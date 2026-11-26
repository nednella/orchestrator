import assert from 'node:assert/strict'
import { test } from 'node:test'
import { EMPTY_CHAT, mergeEvents } from './chatEvents.ts'
import { agentList, approvalKind, buildTimeline } from './chatTimeline.ts'
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

test('a turn with no calls and no thinking keeps its heading, with nothing to open, where it was', () => {
  const events = log(user('u1', 'Hi', 10_000), { kind: 'init', id: 's' }, message('msg_1', 'Hello.'), message('msg_2', 'And more.'), turnEnd(false, 13_500))
  const open = buildTimeline(events.slice(0, -1), {})
  const t = buildTimeline(events, {})
  assert.deepEqual(kinds(open.items), ['user', 'working', 'reply', 'reply', 'thinking'])
  assert.deepEqual(kinds(t.items), ['user', 'work', 'reply', 'reply'], 'the fold takes the working row\'s place, and the rest stay in order')
  assert.equal(only(t.items, 'work')[0].key, only(open.items, 'working')[0].key, 'the heading keeps its key, so its row stays')
  assert.deepEqual(only(t.items, 'work').map((w) => [w.key, w.steps.length]), [['work-u1', 0]])
  const [work] = only(t.items, 'work')
  assert.equal(work.endedAt! - work.startedAt, 3_500)
  assert.deepEqual(only(t.items, 'reply').map((r) => r.text), ['Hello.', 'And more.'])
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
  assert.deepEqual(kinds(open.items), ['user', 'working', 'reply', 'work', 'reply', 'work', 'reply', 'thinking'], 'an open turn stays in order, under the working row')
  assert.deepEqual(only(open.items, 'work').map((w) => [w.endedAt, w.calls.length]), [
    [null, 1],
    [null, 1],
  ])

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

test('the turn that runs is headed by its working row, and its thinking streams into its work', () => {
  const thought = (id: string, text: string): Draft => ({ kind: 'thought', id, text })
  const [u, first, use, result] = log(user('u1', 'Fix it', 10_000), thought('msg_1', '**Reading the router**\n\nIt holds the routes.'), toolUse('t1', 'Read'), toolResult('t1', 'a'))
  const lines = (items: TimelineItem[]) => only(items, 'work').map((w) => [w.key, w.steps.map((st) => (st.kind === 'thought' ? [st.key, st.text, st.live] : st.kind))])

  const started = buildTimeline([u], {}, '', { msg_1: '' }).items
  assert.deepEqual(kinds(started), ['user', 'working', 'work'])
  assert.deepEqual(lines(started), [['work-thought-msg_1-0', [['thought-msg_1-0', '', true]]]], 'a block shows from its start')
  assert.equal(only(started, 'working')[0].startedAt, 10_000)

  const thinking = buildTimeline([u, first, use, result], {}, '', { msg_2: 'Now I can ' }).items
  assert.deepEqual(lines(thinking), [['work-thought-msg_1-0', [['thought-msg_1-0', '**Reading the router**\n\nIt holds the routes.', false], 'call', ['thought-msg_2-0', 'Now I can ', true]]]])

  const [second, reply] = log(thought('msg_2', 'Now I can answer.'), message('msg_2', 'Done.')).map((e, i) => ({ ...e, seq: 5 + i, at: 20_000 + i }))
  const writing = buildTimeline([u, first, use, result, second], { msg_2: 'Do' }).items
  assert.deepEqual(kinds(writing), ['user', 'working', 'work', 'reply'])
  assert.deepEqual(lines(writing)[0][1], [['thought-msg_1-0', '**Reading the router**\n\nIt holds the routes.', false], 'call', ['thought-msg_2-0', 'Now I can answer.', false]], 'the logged thought keeps the key it streamed under')
  assert.equal(only(writing, 'reply')[0].streaming, true)

  const ended = { ...turnEnd(false, 73_000), seq: 7 } as ChatEvent
  const events = [u, first, use, result, second, reply, ended]
  const done = buildTimeline(events, {}).items
  assert.deepEqual(kinds(done), ['user', 'work', 'reply'])
  const [work] = only(done, 'work')
  assert.deepEqual([work.key, work.endedAt! - work.startedAt], ['work-u1', 63_000], 'the fold takes the working row\'s place')
  assert.deepEqual(lines(done)[0][1], lines(writing)[0][1], 'the fold holds the same lines')

  const reloaded = mergeEvents(EMPTY_CHAT, events).state
  assert.deepEqual(buildTimeline(reloaded.events, reloaded.live, '', reloaded.thinking).items, done, 'the log alone gives the fold again')
})

test('a turn that only thought folds its thoughts', () => {
  const t = buildTimeline(log(user('u1', 'Hi'), { kind: 'thought', id: 'msg_1', text: 'A greeting.' }, message('msg_1', 'Hello.'), turnEnd()), {})
  assert.deepEqual(kinds(t.items), ['user', 'work', 'reply'])
  assert.deepEqual(only(t.items, 'work')[0].steps.map((st) => st.kind), ['thought'])
})

test('blocks of one message join', () => {
  const t = buildTimeline(log(user('u1', 'Hi'), message('msg_1', 'First block.'), message('msg_1', 'Second block.'), turnEnd()), {})
  assert.deepEqual(kinds(t.items), ['user', 'work', 'reply'])
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

test('a killed run leaves nothing waiting once the next run starts', () => {
  const t = buildTimeline(log({ kind: 'start' }, user('u1', 'Install it'), toolUse('t1', 'Bash'), approval('r1', 'Bash'), { kind: 'start' }), {}, 'shop/abcd2345')
  assert.equal(t.pending, null)
  assert.equal(t.turnOpen, false)
  assert.equal(only(t.items, 'approval')[0].expired, true)
})

test('claude started again without a flag it did not know goes on with the same turn', () => {
  const t = buildTimeline(log(user('u1', 'Hi'), { kind: 'start', text: '--thinking-display' }, message('msg_1', 'Hello.')), {})
  assert.deepEqual(kinds(t.items), ['user', 'working', 'reply', 'thinking'])
  assert.equal(t.turnOpen, true)
})

test('an unanswered approval expires when Claude withdraws it or rejects its call, not at the turn end', () => {
  const events = log(user('u1', 'Install it'), toolUse('t1', 'Bash', { command: 'npm install\nnpm test' }), approval('r1', 'Bash', { command: 'npm install\nnpm test' }, 't1'))
  const waiting = buildTimeline(events, {}, 'shop/abcd2345')
  assert.deepEqual(waiting.pending, { id: 'shop/abcd2345', request: 'r1', tool: 'Bash', summary: 'npm install', kind: 'tool', input: { command: 'npm install\nnpm test' }, inputTooLarge: false })
  assert.deepEqual(kinds(waiting.items), ['user', 'working', 'approval'])
  assert.equal(only(waiting.items, 'approval')[0].expired, false)

  const after = (...drafts: Draft[]) => buildTimeline(log(...events, ...drafts), {}, 'shop/abcd2345')
  assert.equal(after(turnEnd(true)).pending?.request, 'r1', 'a turn end alone ends no approval')
  for (const [name, ended] of [
    ['withdrawn at an interrupt', after({ kind: 'interrupt' }, { kind: 'cancel', id: 'r1' }, turnEnd(true))],
    ['its call rejected', after(toolResult('t1', 'The user does not want to proceed', true), turnEnd(true))],
  ] as const) {
    assert.deepEqual([ended.pending, ended.turnOpen], [null, false], name)
    const [expired] = only(ended.items, 'approval')
    assert.deepEqual([expired.request, expired.tool, expired.expired], ['r1', 'Bash', true], name)
    assert.equal(only(ended.items, 'work')[0].calls[0].answer, undefined, name)
  }
})

test('a call that waits on its approval shows only as the card', () => {
  const asking = log(user('u1', 'Install it'), toolUse('t1', 'Read', { file_path: 'package.json' }), toolResult('t1', '{}'), toolUse('t2', 'Bash', { command: 'npm install' }), approval('r1', 'Bash', { command: 'npm install' }, 't2'))
  const waiting = buildTimeline(asking, {})
  assert.deepEqual(kinds(waiting.items), ['user', 'working', 'work', 'approval'])
  assert.deepEqual(only(waiting.items, 'work')[0].calls.map((c) => c.id), ['t1'])
  const allowed = buildTimeline(log(...asking, answer('r1', 'allow')), {})
  assert.deepEqual(kinds(allowed.items), ['user', 'working', 'work', 'thinking'])
  assert.deepEqual(only(allowed.items, 'work')[0].calls.map((c) => c.id), ['t1', 't2'])
})

test('question and plan items', () => {
  const questions = { questions: [{ question: 'Which colour do you like?', header: 'Colour', options: [{ label: 'Red' }, { label: 'Blue' }], multiSelect: false }] }
  const plan = { plan: '# Move checkout\n\n1. Add the route' }
  const asking = log(user('u1', 'Ask me'), toolUse('t1', 'AskUserQuestion', questions), approval('r1', 'AskUserQuestion', questions))
  const t = buildTimeline(asking, {})
  assert.deepEqual(kinds(t.items), ['user', 'working', 'question'])
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

test('approvalKind', () => {
  assert.deepEqual(['Bash', 'AskUserQuestion', 'ExitPlanMode', 'mcp__browser__click'].map(approvalKind), ['tool', 'question', 'plan', 'tool'])
})

test('a question keeps the answers its approval allowed with', () => {
  const questions = { questions: [{ question: 'Which colour do you like?', options: [{ label: 'Red' }, { label: 'Blue' }] }] }
  const asking = log(user('u1', 'Ask me'), toolUse('t1', 'AskUserQuestion', questions), approval('r1', 'AskUserQuestion', questions, 't1'))
  const answers = (...drafts: Draft[]) => only(buildTimeline(log(...asking, ...drafts), {}).items, 'question')[0].answers
  assert.equal(answers(), null)
  assert.deepEqual(answers({ kind: 'answer', id: 'r1', text: 'allow', input: { ...questions, answers: { 'Which colour do you like?': 'Red' } } }), { 'Which colour do you like?': 'Red' })
  assert.equal(answers(answer('r1', 'deny')), null, 'a deny, or an older log, has none')
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

test('a settle or a restart the host made is a quiet line; a crash is red', () => {
  const ended = (exit: Draft) => buildTimeline(log(user('u1', 'Go'), message('msg_1', 'On it.'), turnEnd(), exit), {}).items.at(-1)
  assert.deepEqual(ended({ kind: 'exit', stop: 'settle' }), { kind: 'notice', key: 'notice-4', at: 4000, text: 'Asleep', error: false })
  assert.deepEqual(ended({ kind: 'exit', stop: 'restart', text: 'full' }), { kind: 'restart', key: 'notice-4', at: 4000, access: 'full' })
  assert.deepEqual(ended({ kind: 'exit', text: 'exit status 143', error: true }), { kind: 'notice', key: 'notice-4', at: 4000, text: 'Claude stopped', error: true })
})

test('a failed turn says why when Claude does', () => {
  const notices = only(buildTimeline(log(user('u1', 'Go'), { kind: 'turn_end', error: true, text: 'No message found with message.uuid of: r9' }), {}).items, 'notice')
  assert.deepEqual(notices.map((n) => [n.text, n.error]), [['No message found with message.uuid of: r9', true]])
  const failed = (text: string) => only(buildTimeline(log(user('u1', 'Go'), { kind: 'turn_end', error: true, text }), {}).items, 'notice')[0].text
  assert.equal(failed('[ede_diagnostic] result_type=user last_content_type=n/a stop_reason=null'), 'The turn ended with an error', 'only a diagnostic line says nothing')
  assert.equal(failed('[ede_diagnostic] result_type=user\nNo conversation found with session ID: abc'), 'No conversation found with session ID: abc')
})

test('a reply forks at its last block, and a message at the reply before it', () => {
  const events = log(
    user('u1', 'First'),
    { kind: 'message', id: 'msg_1', uuid: 'r1a', text: 'One.' },
    { kind: 'message', id: 'msg_1', uuid: 'r1b', text: 'Two.' },
    turnEnd(),
    user('u2', 'Second'),
    toolUse('t1', 'Read'),
    toolResult('t1', 'a'),
    { kind: 'message', id: 'msg_2', uuid: 'r2', text: 'Three.' },
    turnEnd(),
    user('u3', 'Third'),
    message('msg_3', 'From an older log.'),
    turnEnd(),
    user('u4', 'Fourth'),
  )
  const items = buildTimeline(events, {}).items
  assert.deepEqual(
    only(items, 'reply').map((r) => [r.text, r.uuid]),
    [
      ['One.\n\nTwo.', 'r1b'],
      ['Three.', 'r2'],
      ['From an older log.', ''],
    ],
  )
  assert.deepEqual(
    only(items, 'user').map((u) => [u.text, u.forkAt]),
    [
      ['First', ''],
      ['Second', 'r1b'],
      ['Third', 'r2'],
      ['Fourth', ''],
    ],
  )
  const streaming = buildTimeline(log(user('u1', 'Go')), { msg_1: 'Wor' }).items
  assert.equal(only(streaming, 'reply')[0].uuid, '', 'a reply that only streamed has no uuid yet')
})

test('no fork ends at a message from before the last compaction', () => {
  const events = log(
    user('u1', 'First'),
    { kind: 'message', id: 'msg_1', uuid: 'r1', text: 'One.' },
    turnEnd(),
    user('u2', '/compact'),
    { kind: 'compact' },
    turnEnd(),
    user('u3', 'Second'),
    { kind: 'message', id: 'msg_2', uuid: 'r2', text: 'Two.' },
    turnEnd(),
    user('u4', '/compact'),
    { kind: 'compact' },
    turnEnd(),
    user('u5', 'Third'),
    { kind: 'message', id: 'msg_3', uuid: 'r3', text: 'Three.' },
    turnEnd(),
    user('u6', 'Fourth'),
  )
  const items = buildTimeline(events, {}).items
  assert.deepEqual(
    only(items, 'reply').map((r) => [r.text, r.uuid]),
    [
      ['One.', ''],
      ['Two.', ''],
      ['Three.', 'r3'],
    ],
  )
  assert.deepEqual(
    only(items, 'user').map((u) => [u.text, u.forkAt]),
    [
      ['First', ''],
      ['/compact', ''],
      ['Second', ''],
      ['/compact', ''],
      ['Third', ''],
      ['Fourth', 'r3'],
    ],
  )
})

test('queued until delivered', () => {
  const sent = log(user('u1', 'Slow job'), message('msg_1', 'Working.'), { kind: 'queued', id: 'q1', text: 'Also run the tests', files: ['/m/a.png'] })
  const waiting = buildTimeline(sent, {})
  assert.deepEqual(waiting.queued, [{ id: 'q1', text: 'Also run the tests', files: ['/m/a.png'] }])
  assert.deepEqual(kinds(waiting.items), ['user', 'working', 'reply', 'thinking'])

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

test('a message a failed start never read is not sent, as an error', () => {
  const t = buildTimeline(log({ kind: 'start' }, user('u1', 'Hi\nthere'), { kind: 'unsent', id: 'u1', text: 'Hi\nthere', error: true }, { kind: 'exit', error: true, text: 'exit status 1' }), {})
  assert.deepEqual(kinds(t.items), ['user', 'work', 'notice', 'notice'])
  const notices = only(t.items, 'notice')
  assert.deepEqual(notices.map((n) => [n.key, n.error]).at(-1), ['unsent-u1', true])
  assert.equal(notices.at(-1)?.text, 'Not sent: Hi')
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

test('a reply shows the time of its last block', () => {
  const t = buildTimeline(log(user('u1', 'Hi'), message('msg_1', 'One.'), message('msg_1', 'Two.'), turnEnd()), {})
  assert.equal(only(t.items, 'reply')[0].at, 3000)
})

test('the reply a turn stopped or failed on is marked, and no other', () => {
  const endedOf = (...drafts: Draft[]) => only(buildTimeline(log(...drafts), {}).items, 'reply').map((r) => r.ended)
  assert.deepEqual(endedOf(user('u1', 'Go'), message('msg_1', 'On it.'), turnEnd()), [undefined])
  assert.deepEqual(endedOf(user('u1', 'Go'), message('msg_1', 'On it.'), { kind: 'interrupt' }, turnEnd(true)), ['stopped'])
  assert.deepEqual(endedOf(user('u1', 'Go'), message('msg_1', 'On it.'), turnEnd(true)), ['failed'])
  assert.deepEqual(endedOf(user('u1', 'Go'), message('msg_1', 'On it.'), { kind: 'exit' }), ['stopped'])
  assert.deepEqual(endedOf(user('u1', 'Go'), message('msg_1', 'On it.'), { kind: 'exit', error: true }), ['failed'])
  assert.deepEqual(endedOf(user('u1', 'Go'), message('msg_1', 'On it.'), { kind: 'start' }), ['stopped'])
  assert.deepEqual(endedOf(user('u1', 'Go'), message('msg_1', 'On it.'), toolUse('t1', 'Bash'), { kind: 'interrupt' }, turnEnd(true)), [undefined], 'the turn stopped on the call, after the reply')
  assert.deepEqual(endedOf(user('u1', 'Go'), message('msg_1', 'On it.'), user('u2', 'And this'), message('msg_2', 'Sure.'), { kind: 'interrupt' }, turnEnd(true)), [undefined, 'stopped'])
})

const agentCall = (id: string, description: string, type = 'general-purpose'): Draft => ({ kind: 'tool_use', id, tool: 'Agent', input: { description, subagent_type: type, prompt: `Do: ${description}` } })
const taskEnd = (parent: string, text: string, how: 'completed' | 'failed' | 'stopped' = 'completed'): Draft => ({ kind: 'task_end', parent, text, ...(how !== 'completed' ? { error: true } : {}), ...(how === 'stopped' ? { stop: 'stopped' as const } : {}) })
const from = (agent: string, d: Draft): Draft => ({ ...d, agent })

// Two subagents in the background, as Claude runs them by default: the calls' results come at once and the turn ends,
// then the subagents work and end.
const parallel = log(
  user('u1', 'List the files and count the lines', 10_000),
  agentCall('a1', 'List files', 'Explore'),
  agentCall('a2', 'Count lines'),
  toolResult('a1', 'Async agent launched successfully.'),
  toolResult('a2', 'Async agent launched successfully.'),
  message('msg_1', 'Both are running.'),
  turnEnd(false, 15_000),
  from('a2', toolUse('s2', 'Bash', { command: 'wc -l README.md' })),
  from('a1', { kind: 'thought', id: 'msg_s1', text: 'I will list them.' }),
  from('a1', toolUse('s1', 'Bash', { command: 'ls' })),
  from('a1', toolResult('s1', 'a.go b.go')),
  from('a1', message('msg_s1', 'a.go, b.go')),
  { ...taskEnd('a1', 'a.go, b.go'), at: 21_000 },
)

test('parallel subagents are rows of their own beside the fold, holding their own events', () => {
  const t = buildTimeline(parallel, {})
  assert.deepEqual(kinds(t.items), ['user', 'work', 'agent', 'agent', 'reply'])
  assert.equal(t.turnOpen, false, "a subagent's events after its turn opens no turn")
  const [work] = only(t.items, 'work')
  assert.deepEqual(work.steps, [], 'the Agent calls are not tool rows of the fold')
  const [first, second] = only(t.items, 'agent')
  assert.deepEqual([first.key, first.description, first.type, first.state, first.startedAt, first.endedAt, first.tools, first.result], ['agent-a1', 'List files', 'Explore', 'done', 2_000, 21_000, 1, 'a.go, b.go'])
  assert.deepEqual(
    first.steps.map((st) => (st.kind === 'call' ? [st.call.id, st.call.result] : [st.kind, st.text])),
    [
      ['thought', 'I will list them.'],
      ['s1', 'a.go b.go'],
    ],
    'its last text is its report, which shows once',
  )
  assert.deepEqual([second.type, second.state, second.endedAt, second.tools, second.result], ['', 'running', null, 1, null], 'the general type is not named')
  assert.equal(second.turn, first.turn)
})

test('subagents in the background run on after their turn', () => {
  const t = buildTimeline(parallel, {})
  assert.deepEqual([t.turnOpen, t.agentsRunning], [false, true], 'one still works after the turn ended')
  const done = buildTimeline([...parallel, { ...taskEnd('a2', '5'), seq: parallel.length + 1, at: 22_000 }], {})
  assert.equal(done.agentsRunning, false)
  assert.equal(buildTimeline(log(user('u1', 'Hi'), message('msg_1', 'Hello.'), turnEnd()), {}).agentsRunning, false)
})

test('subagents of the turn that runs split its work like a card', () => {
  const t = buildTimeline(log(user('u1', 'Go'), toolUse('t1', 'Read'), agentCall('a1', 'Look'), from('a1', toolUse('s1', 'Grep')), toolUse('t2', 'Read')), {})
  assert.deepEqual(kinds(t.items), ['user', 'working', 'work', 'agent', 'work', 'thinking'])
  assert.deepEqual(only(t.items, 'work').map((w) => w.steps.length), [1, 1], "the subagent's call is not one of the turn's")
  assert.equal(only(t.items, 'agent')[0].tools, 1)
})

test('a subagent ends as its end says, and stops with its Claude', () => {
  const ended = (how: 'completed' | 'failed' | 'stopped') => only(buildTimeline(log(user('u1', 'Go'), agentCall('a1', 'Look'), taskEnd('a1', 'why', how)), {}).items, 'agent')[0]
  assert.deepEqual([ended('failed').state, ended('failed').result], ['failed', 'why'])
  assert.equal(ended('stopped').state, 'stopped')
  const exited = only(buildTimeline(log(user('u1', 'Go'), agentCall('a1', 'Look'), turnEnd(), { kind: 'exit', at: 9_000 }), {}).items, 'agent')[0]
  assert.deepEqual([exited.state, exited.endedAt], ['stopped', 9_000])
  const refused = only(buildTimeline(log(user('u1', 'Go'), agentCall('a1', 'Look'), toolResult('a1', 'No such agent type', true)), {}).items, 'agent')[0]
  assert.deepEqual([refused.state, refused.result], ['failed', 'No such agent type'])
  const late = only(buildTimeline(log(user('u1', 'Go'), agentCall('a1', 'Look'), taskEnd('a1', 'done'), taskEnd('a1', 'again', 'failed')), {}).items, 'agent')[0]
  assert.deepEqual([late.state, late.result], ['done', 'done'], 'the first end holds')
})

test("an event of a subagent with no call shows nowhere, and a shell's end changes nothing", () => {
  const t = buildTimeline(log(user('u1', 'Go'), message('msg_1', 'Hi'), turnEnd(), from('gone', message('msg_2', 'lost')), taskEnd('t9', 'shell done')), {})
  assert.deepEqual(kinds(t.items), ['user', 'work', 'reply'])
})

const taskStart = (parent: string, kind = 'local_agent'): Draft => ({ kind: 'task_start', parent, id: `task-${parent}`, tool: kind })

// A subagent in the background as the host logs it: its task starts with the call, the turn ends at its launch, it
// works and ends, and Claude tells of its end in a turn of its own.
const background = log(
  user('u1', 'List the files', 10_000),
  agentCall('a1', 'List files', 'Explore'),
  taskStart('a1'),
  toolResult('a1', 'Async agent launched successfully.'),
  message('msg_1', 'It is running.'),
  turnEnd(false, 15_000),
  from('a1', toolUse('s1', 'Bash', { command: 'ls' })),
  from('a1', toolResult('s1', 'a.go')),
  { ...taskEnd('a1', 'a.go'), at: 21_000 },
  { ...message('msg_2', 'The subagent found a.go.'), at: 22_000 },
  turnEnd(false, 23_000),
)

test('the turns are one piece of work until Claude has told of the end of its subagents', () => {
  const at = (n: number) => buildTimeline(background.slice(0, n), {})
  const rests = at(6)
  assert.equal(rests.turnOpen, false, 'a message sent now starts a turn')
  assert.deepEqual(kinds(rests.items), ['user', 'working', 'agent', 'reply', 'thinking'], 'the work goes on while the subagent runs')
  assert.deepEqual(kinds(at(9).items), ['user', 'working', 'agent', 'reply', 'thinking'], 'and until Claude tells of its end')
  assert.equal(at(10).turnOpen, true, "Claude's own turn runs")
  const t = at(11)
  assert.deepEqual(kinds(t.items), ['user', 'work', 'agent', 'reply', 'reply'], "Claude's own turn shows no message of the owner's")
  const [work] = only(t.items, 'work')
  assert.deepEqual([work.startedAt, work.endedAt, t.turnOpen], [10_000, 23_000, false])
  assert.deepEqual(only(t.items, 'reply').map((r) => r.text), ['It is running.', 'The subagent found a.go.'])
})

test('a piece of work that waits on its subagents ends where the owner, an error or an exit ends it', () => {
  const rest = background.slice(0, 6)
  const after = (...drafts: Draft[]) => buildTimeline([...rest, ...log(...drafts).map((e) => ({ ...e, seq: e.seq + rest.length, at: e.at + 30_000 }))], {})
  const sent = after(user('u2', 'Also count them'))
  assert.deepEqual(kinds(sent.items), ['user', 'work', 'agent', 'reply', 'user', 'working', 'thinking'])
  assert.deepEqual(only(sent.items, 'work').map((w) => w.endedAt), [15_000], 'the owner\'s message ends the piece where its turn ended')
  const stopped = after(taskEnd('a1', '', 'stopped'))
  assert.deepEqual([only(stopped.items, 'work')[0].endedAt, stopped.turnOpen], [15_000, false], 'Claude tells of no stopped task')
  const exited = after({ kind: 'exit' })
  assert.deepEqual([only(exited.items, 'work')[0].endedAt, only(exited.items, 'agent')[0].state], [31_000, 'stopped'])
  const failed = buildTimeline(log(user('u1', 'Go'), agentCall('a1', 'Look'), taskStart('a1'), turnEnd(true, 9_000)), {})
  assert.equal(only(failed.items, 'work')[0].endedAt, 9_000)
})

test("Claude's list of its tasks ends the piece when a subagent's end was missed, and holds it for a task only it names", () => {
  const listed = (...tasks: { id: string; kind: string }[]): Draft => ({ kind: 'tasks', input: tasks })
  const rest = background.slice(0, 6)
  const missed = buildTimeline([...rest, { ...listed(), seq: 7, at: 40_000 }], {})
  assert.deepEqual([only(missed.items, 'work')[0].endedAt, missed.turnOpen], [15_000, false], 'Claude tells of no end it never sent')
  const shellOnly = buildTimeline([...rest, { ...listed({ id: 'b9', kind: 'local_bash' }), seq: 7, at: 40_000 }], {})
  assert.equal(only(shellOnly.items, 'work')[0].endedAt, 15_000, 'a shell is no work')
  const monitorOnly = buildTimeline([...rest, { ...listed({ id: 'm9', kind: 'monitor_mcp' }), seq: 7, at: 40_000 }], {})
  assert.equal(only(monitorOnly.items, 'work')[0].endedAt, 15_000, 'a monitor is no work')
  const unknown = buildTimeline(log(user('u1', 'Go', 1_000), listed({ id: 'a7', kind: 'local_agent' }), message('msg_1', 'On it.'), turnEnd(false, 3_000)), {})
  assert.deepEqual(kinds(unknown.items), ['user', 'working', 'reply', 'thinking'], 'a subagent only the list names holds the work')
  const started = buildTimeline(log(user('u1', 'Go'), agentCall('a1', 'Look'), listed({ id: 'task-a1', kind: 'local_agent' }), taskStart('a1'), turnEnd(), taskEnd('a1', 'done', 'stopped')), {})
  assert.equal(only(started.items, 'work')[0].endedAt, 5_000, 'the start after the list is the same task, which its end ends')
})

test('a shell or a monitor in the background does not hold the work open', () => {
  const t = buildTimeline(log(user('u1', 'Serve it', 1_000), toolUse('b1', 'Bash', { command: 'npm start', run_in_background: true }), taskStart('b1', 'local_bash'), toolResult('b1', 'Running'), turnEnd(false, 4_000)), {})
  assert.equal(only(t.items, 'work')[0].endedAt, 4_000)
  const m = buildTimeline(log(user('u1', 'Watch it', 1_000), toolUse('m1', 'Monitor', {}), taskStart('m1', 'monitor_mcp'), toolResult('m1', 'Watching'), turnEnd(false, 4_000)), {})
  assert.equal(only(m.items, 'work')[0].endedAt, 4_000, 'a monitor')
})

test("a subagent's own task holds the work open, and a subagent woken again runs", () => {
  const t = buildTimeline(log(user('u1', 'Go'), agentCall('a1', 'Look'), taskStart('a1'), turnEnd(), from('a1', taskStart('b2')), taskEnd('a1', 'done'), taskStart('a1')), {})
  assert.deepEqual(kinds(t.items), ['user', 'working', 'agent', 'thinking'])
  assert.equal(only(t.items, 'agent')[0].state, 'running')
})

// A subagent in the background asks for an approval after the turn that started it ended.
const asking = log(
  user('u1', 'Write the file in a subagent'),
  agentCall('a1', 'Write file'),
  taskStart('a1'),
  toolResult('a1', 'Async agent launched successfully.'),
  turnEnd(),
  from('a1', toolUse('s1', 'Write', { file_path: 'sub.txt' })),
  from('a1', approval('r1', 'Write', { file_path: 'sub.txt' }, 's1')),
)

test("a subagent's approval shows after its row and waits past the turns until it is answered", () => {
  const t = buildTimeline(asking, {}, 'main/t1')
  assert.deepEqual(kinds(t.items), ['user', 'working', 'agent', 'approval'])
  assert.deepEqual([t.pending?.request, t.pending?.tool, t.pending?.summary], ['r1', 'Write', 'sub.txt'])
  const later = buildTimeline([...asking, ...log(user('u2', 'Meanwhile, hi'), message('msg_2', 'Hi.'), turnEnd()).map((e) => ({ ...e, seq: e.seq + asking.length }))], {}, 'main/t1')
  assert.deepEqual(kinds(later.items), ['user', 'work', 'agent', 'approval', 'user', 'working', 'reply'], "another turn's end leaves it")
  assert.equal(only(later.items, 'approval')[0].expired, false)
  assert.equal(later.pending?.request, 'r1')
  const answered = buildTimeline([...asking, ...log(answer('r1', 'allow'), from('a1', toolResult('s1', 'Wrote sub.txt'))).map((e) => ({ ...e, seq: e.seq + asking.length }))], {}, 'main/t1')
  assert.deepEqual(kinds(answered.items), ['user', 'working', 'agent', 'thinking'])
  assert.equal(answered.pending, null)
  const [step] = only(answered.items, 'agent')[0].steps
  assert.deepEqual(step.kind === 'call' && [step.call.answer, step.call.result], ['allow', 'Wrote sub.txt'])
})

test("a subagent's approval can no longer be answered once its call has a result, the subagent ends or its Claude goes", () => {
  const after = (d: Draft) => buildTimeline([...asking, { at: 99_000, ...d, seq: asking.length + 1 }], {})
  for (const [name, d] of [
    ['its call has a result', from('a1', toolResult('s1', 'denied', true))],
    ['the subagent ends', taskEnd('a1', '', 'stopped')],
    ["Claude's list of its tasks leaves the subagent out", { kind: 'tasks', input: [] }],
    ['its Claude exits', { kind: 'exit' }],
    ['a new run starts', { kind: 'start' }],
  ] as [string, Draft][]) {
    const t = after(d)
    assert.deepEqual([only(t.items, 'approval')[0]?.expired, t.pending], [true, null], name)
  }
  const listed = after({ kind: 'tasks', input: [{ id: 'task-a1', kind: 'local_agent' }] })
  assert.deepEqual([only(listed.items, 'approval')[0]?.expired, listed.pending?.request], [false, 'r1'], "a list that names the subagent leaves it")
})

test("a message that denies a subagent's approval is its answer, not a turn of the thread's", () => {
  const t = buildTimeline([...asking, ...log(answer('r1', 'deny'), from('a1', { ...user('u2', 'Use b.txt'), parent: 'r1' }), from('a1', toolResult('s1', 'denied', true))).map((e) => ({ ...e, seq: e.seq + asking.length }))], {})
  assert.deepEqual(kinds(t.items), ['user', 'work', 'agent', 'user'], "the owner's text shows where the owner wrote it")
  assert.equal(only(t.items, 'user')[1].text, 'Use b.txt')
  assert.deepEqual([t.turnOpen, t.pending], [false, null])
  const during = buildTimeline(log(user('u1', 'Go'), agentCall('a1', 'Look'), from('a1', toolUse('s1', 'Write')), from('a1', approval('r1', 'Write', {}, 's1')), answer('r1', 'deny'), from('a1', { ...user('u2', 'Not that file'), parent: 'r1' })), {})
  assert.deepEqual([kinds(during.items), during.turnOpen], [['user', 'work', 'agent', 'user', 'working', 'thinking'], true], 'the turn that waits on its subagent goes on after it')
})

test("a subagent's approval with no row of its own still shows, and waits", () => {
  const unknown = buildTimeline([...asking.slice(0, -1), { ...from('gone', approval('r1', 'Write', { file_path: 'sub.txt' }, 's9')), seq: asking.length, at: 9_000 }], {}, 'main/t1')
  assert.deepEqual(kinds(unknown.items), ['user', 'working', 'agent', 'approval'])
  assert.equal(unknown.pending?.request, 'r1')
  const nested = buildTimeline([...asking.slice(0, -1), ...log(from('a1', agentCall('n1', 'Look deeper')), from('n1', approval('r2', 'Bash', { command: 'ls' }, 's2')), message('msg_9', 'Still here.')).map((e) => ({ ...e, seq: e.seq + asking.length }))], {}, 'main/t1')
  assert.deepEqual(kinds(nested.items), ['user', 'working', 'agent', 'approval', 'reply'], "a subagent's subagent asks under the row it shows in")
  assert.equal(nested.pending?.request, 'r2')
})

test('an approval in a log from before the host named the subagent finds it by its call', () => {
  const unnamed = asking.map((e) => (e.kind === 'approval' ? { ...e, agent: undefined } : e))
  assert.deepEqual(kinds(buildTimeline(unnamed, {}).items), ['user', 'working', 'agent', 'approval'])
})

test('the agent list puts the newest turn first', () => {
  const events = log(user('u1', 'One'), agentCall('a1', 'First'), agentCall('a2', 'Second'), turnEnd(), user('u2', 'Two'), agentCall('a3', 'Third'), turnEnd())
  assert.deepEqual(
    agentList(buildTimeline(events, {}).items).map((a) => a.id),
    ['a3', 'a1', 'a2'],
  )
})

test('a rewind drops its message and everything after it, up to the rewind', () => {
  const events = log(
    user('u1', 'First'),
    { kind: 'message', id: 'msg_1', uuid: 'r1', text: 'One.' },
    turnEnd(),
    user('u2', 'Second'),
    toolUse('t1', 'Edit', { file_path: 'a.ts' }),
    toolResult('t1', 'done'),
    { kind: 'message', id: 'msg_2', uuid: 'r2', text: 'Two.' },
    turnEnd(),
    { kind: 'exit', stop: 'settle' },
    { kind: 'start' },
    user('u3', 'Third'),
    { kind: 'message', id: 'msg_3', uuid: 'r3', text: 'Three.' },
    turnEnd(),
    { kind: 'rewind', id: 'u2' },
    user('u4', 'Second again'),
    { kind: 'message', id: 'msg_4', uuid: 'r4', text: 'Four.' },
    turnEnd(),
  )
  const t = buildTimeline(events, {})
  assert.deepEqual(kinds(t.items), ['user', 'work', 'reply', 'user', 'work', 'reply'])
  assert.deepEqual(
    only(t.items, 'user').map((u) => [u.text, u.forkAt]),
    [
      ['First', ''],
      ['Second again', 'r1'],
    ],
    'the message after the rewind forks at the reply before the rewound one',
  )
  assert.equal(t.turnOpen, false)
  assert.deepEqual(kinds(buildTimeline(log(user('u1', 'First'), turnEnd(), { kind: 'rewind', id: 'u1' }), {}).items), [], 'a rewind to the first message leaves nothing')
  assert.deepEqual(kinds(buildTimeline(log(user('u1', 'First'), turnEnd(), { kind: 'rewind', id: 'u9' }), {}).items), ['user', 'work'], 'a rewind to a message the chat does not show drops nothing')
})

test('only a message Claude holds as sent can be rewound to', () => {
  const events = log(
    user('u1', 'First'),
    turnEnd(),
    user('u2', 'Fix it'),
    toolUse('t1', 'Bash', { command: 'rm -r build' }),
    approval('a1', 'Bash', { command: 'rm -r build' }, 't1'),
    answer('a1', 'deny'),
    { kind: 'user', id: 'u3', text: 'Keep the build', parent: 'a1' },
    turnEnd(),
    user('u4', '/compact'),
    { kind: 'compact' },
    turnEnd(),
    user('u5', 'Then this'),
  )
  assert.deepEqual(
    only(buildTimeline(events, {}).items, 'user').map((u) => [u.text, u.uuid]),
    [
      ['First', ''],
      ['Fix it', ''],
      ['Keep the build', ''],
      ['/compact', ''],
      ['Then this', 'u5'],
    ],
  )
  assert.deepEqual(only(buildTimeline(events.slice(0, 8), {}).items, 'user').map((u) => u.uuid), ['u1', 'u2', ''], 'a denial is no message of the conversation')
  const denied = events.slice(0, 8)
  assert.deepEqual(kinds(buildTimeline([...denied, { kind: 'rewind', id: 'u3', seq: 99, at: 99_000 }], {}).items), kinds(buildTimeline(denied, {}).items), 'a rewind never cuts at a denial')
})

const delegate = (id: string, input: unknown = { task: 'Find the flaky test', title: 'Flaky test' }): Draft => toolUse(id, 'mcp__agentos__delegate', input)
const started = (parent: string, child: string): Draft => toolResult(parent, JSON.stringify({ id: child, n: 7, title: 'Flaky test', project: 'shop', access: 'read', where: 'folder', dir: '/code/shop', branch: '' }))

test('delegate call becomes a child card', () => {
  const events = log(user('u1', 'Split it up'), toolUse('t1', 'Read', { file_path: 'a.ts' }), toolResult('t1', 'x'), delegate('d1'), message('msg_1', 'Started a child.'))
  const open = buildTimeline(events, {})
  assert.deepEqual(kinds(open.items), ['user', 'working', 'work', 'child', 'reply', 'thinking'], 'the card is not in the work row')
  assert.deepEqual(only(open.items, 'child'), [{ kind: 'child', key: 'child-d1', at: 4000, call: 'd1', child: '', title: 'Flaky test', error: false, expired: false }])
  const t = buildTimeline(log(...events.map(({ seq: _s, at: _a, ...d }) => d), turnEnd()), {})
  assert.deepEqual(kinds(t.items), ['user', 'work', 'child', 'reply'], 'a finished turn keeps the card outside the fold')
  assert.deepEqual(only(t.items, 'work')[0].calls.map((c) => c.tool), ['Read'])
  assert.equal(only(buildTimeline(log(user('u1', 'Go'), delegate('d1', { task: 'Look at the cart\nand more' })), {}).items, 'child')[0].title, 'Look at the cart', 'with no title, the task names it')
})

test('child card takes the id from the result', () => {
  const t = buildTimeline(log(user('u1', 'Go'), delegate('d1'), started('d1', 'shop/abcd2345'), turnEnd()), {})
  assert.equal(only(t.items, 'child')[0].child, 'shop/abcd2345')
  const odd = buildTimeline(log(user('u1', 'Go'), delegate('d1'), toolResult('d1', 'not json'), turnEnd()), {})
  assert.equal(only(odd.items, 'child')[0].child, '', 'a result with no id leaves it empty')
})

test('failed delegate stays a tool row', () => {
  const events = log(user('u1', 'Go'), delegate('d1'), toolResult('d1', 'This thread has 8 children running', true), message('msg_1', 'It failed.'))
  const open = buildTimeline(events, {})
  assert.deepEqual(kinds(open.items), ['user', 'working', 'work', 'reply', 'thinking'])
  const [work] = only(open.items, 'work')
  assert.deepEqual(work.calls.map((c) => [c.tool, c.result, c.error]), [['mcp__agentos__delegate', 'This thread has 8 children running', true]])
  const t = buildTimeline(log(...events.map(({ seq: _s, at: _a, ...d }) => d), turnEnd()), {})
  assert.deepEqual(kinds(t.items), ['user', 'work', 'reply'])
  assert.deepEqual(only(t.items, 'work')[0].calls.map((c) => c.error), [true])
})

test('child result becomes a result card', () => {
  const t = buildTimeline(
    log(user('u1', 'Go'), delegate('d1'), started('d1', 'shop/abcd2345'), turnEnd(), { kind: 'user', id: 'u2', text: 'APPLICATION_PROMPT: Task Flaky test is done:\n\nThe retry test.', from: 'shop/abcd2345' }, message('msg_2', 'The child found it.')),
    {},
  )
  assert.deepEqual(kinds(t.items), ['user', 'work', 'child', 'childResult', 'working', 'reply', 'thinking'])
  assert.deepEqual(only(t.items, 'childResult'), [{ kind: 'childResult', key: 'result-u2', at: 5000, child: 'shop/abcd2345', text: 'Task Flaky test is done:\n\nThe retry test.' }])
  assert.equal(t.turnOpen, true, 'the result starts a turn, as a message does')
})

test('message from another thread is headed with its sender', () => {
  const t = buildTimeline(log({ kind: 'user', id: 'u1', text: 'APPLICATION_PROMPT: Look at the cart too.', from: 'shop/other234' }, turnEnd(), user('u2', 'Mine')), {})
  const [theirs, mine] = only(t.items, 'user')
  assert.equal(theirs.from, 'shop/other234')
  assert.equal(theirs.text, 'Look at the cart too.')
  assert.equal(mine.from, undefined, "the owner's message names no sender")
})

test('a delegate call that gets no answer ends with its turn', () => {
  const asked = [user('u1', 'Go'), delegate('d1')]
  assert.equal(only(buildTimeline(log(...asked), {}).items, 'child')[0].expired, false, 'it waits while its turn runs')
  for (const end of [{ kind: 'exit', error: true } as Draft, turnEnd(true), { kind: 'start' } as Draft]) {
    assert.equal(only(buildTimeline(log(...asked, end), {}).items, 'child')[0].expired, true, `${end.kind} ends it`)
  }
  const answered = buildTimeline(log(...asked, started('d1', 'shop/abcd2345'), turnEnd()), {})
  assert.equal(only(answered.items, 'child')[0].expired, false, 'one that got its answer stays')
  const split = buildTimeline(log(...asked, user('u2', 'And this')), {})
  assert.equal(only(split.items, 'child')[0].expired, false, 'a message that splits the turn does not end it')
})

test('a delegate result that comes after its turn ended still names the child', () => {
  const [item] = only(buildTimeline(log(user('u1', 'Go'), delegate('d1'), turnEnd(true), started('d1', 'shop/abcd2345')), {}).items, 'child')
  assert.deepEqual([item.child, item.expired], ['shop/abcd2345', false])
})

const live = (items: TimelineItem[]) => items.filter((i) => i.kind === 'working' || i.kind === 'thinking').map((i) => (i.kind === 'thinking' ? [i.kind, i.label] : [i.kind, i.key]))

test('the working row follows the user message', () => {
  const events = log(user('u1', 'Fix it', 10_000), toolUse('t1', 'Read'), toolResult('t1', 'a'), message('msg_1', 'Done.'), turnEnd(false, 20_000))
  const open = buildTimeline(events.slice(0, -1), {}).items
  assert.deepEqual(kinds(open), ['user', 'working', 'work', 'reply', 'thinking'])
  const [working] = only(open, 'working')
  assert.deepEqual([working.key, working.startedAt], ['work-u1', 10_000])
  const done = buildTimeline(events, {}).items
  assert.equal(only(done, 'work')[0].key, working.key, 'the fold takes the working row\'s place')
})

test('the thinking row is last while nothing streams', () => {
  const calling = log(user('u1', 'Fix it'), toolUse('t1', 'Read'))
  assert.deepEqual(buildTimeline(calling, {}).items.at(-1), { kind: 'thinking', key: 'thinking-u1', label: 'Thinking' }, 'a call that runs is no stream')
  const replied = buildTimeline(log(user('u1', 'Fix it'), message('msg_1', 'On it.')), {}).items
  assert.deepEqual(kinds(replied), ['user', 'working', 'reply', 'thinking'], 'a logged reply streams no more')
  const unsent = buildTimeline(log(user('u1', 'Fix it'), { kind: 'queued', id: 'q1', text: 'And this' }, { kind: 'unsent', id: 'q1' }), {}).items
  assert.equal(unsent.at(-1)?.kind, 'thinking', 'after the not-sent notices too')
})

test('no thinking row while a reply streams', () => {
  const t = buildTimeline(log(user('u1', 'Hi')), { msg_1: 'Hel' })
  assert.deepEqual(kinds(t.items), ['user', 'working', 'reply'])
  assert.deepEqual(live(t.items), [['working', 'work-u1']])
})

test('no thinking row while a thought streams', () => {
  const t = buildTimeline(log(user('u1', 'Hi')), {}, '', { msg_1: '' })
  assert.deepEqual(kinds(t.items), ['user', 'working', 'work'])
})

test('no thinking row while an approval waits', () => {
  const asking = log(user('u1', 'Install it'), toolUse('t1', 'Bash'), approval('r1', 'Bash', {}, 't1'))
  assert.deepEqual(kinds(buildTimeline(asking, {}).items), ['user', 'working', 'approval'])
  assert.equal(buildTimeline(log(...asking, answer('r1', 'allow')), {}).items.at(-1)?.kind, 'thinking', 'it comes back once answered')
  const question = { questions: [{ question: 'Which colour?', options: [{ label: 'Red' }] }] }
  assert.deepEqual(kinds(buildTimeline(log(user('u1', 'Ask me'), toolUse('t1', 'AskUserQuestion', question), approval('r1', 'AskUserQuestion', question)), {}).items), ['user', 'working', 'question'])
  assert.deepEqual(kinds(buildTimeline(log(user('u1', 'Plan it'), toolUse('t1', 'ExitPlanMode', { plan: 'x' }), approval('r1', 'ExitPlanMode', { plan: 'x' })), {}).items), ['user', 'working', 'plan'])
  const subagent = buildTimeline(log(user('u1', 'Go'), agentCall('a1', 'Write file'), from('a1', toolUse('s1', 'Write')), from('a1', approval('r1', 'Write', {}, 's1'))), {})
  assert.deepEqual(kinds(subagent.items), ['user', 'working', 'agent', 'approval'], "a subagent's approval waits on the owner too")
})

test('no live rows once the turn ends', () => {
  const ran = log(user('u1', 'Fix it'), toolUse('t1', 'Read'), toolResult('t1', 'a'), message('msg_1', 'Done.'))
  for (const [name, end] of [
    ['finished', [turnEnd()]],
    ['stopped', [{ kind: 'interrupt' }, turnEnd(true)]],
    ['failed', [turnEnd(true)]],
    ['exited', [{ kind: 'exit' }]],
  ] as [string, Draft[]][]) {
    assert.deepEqual(live(buildTimeline(log(...ran, ...end), {}).items), [], name)
  }
})

test('waiting on two subagents', () => {
  const launched = log(user('u1', 'Look around'), agentCall('a1', 'List files'), taskStart('a1'), agentCall('a2', 'Count lines'), taskStart('a2'))
  assert.deepEqual(live(buildTimeline(launched, {}).items).at(-1), ['thinking', 'Thinking'], 'Claude still works on its turn')
  const rests = log(...launched, message('msg_1', 'Both are running.'), turnEnd())
  assert.deepEqual(buildTimeline(rests, {}).items.at(-1), { kind: 'thinking', key: 'thinking-u1', label: 'Waiting on 2 subagents' })
  assert.deepEqual(live(buildTimeline(log(...rests, taskEnd('a1', 'a.go')), {}).items).at(-1), ['thinking', 'Waiting on a subagent'])
  const ended = log(...rests, taskEnd('a1', 'a.go'), taskEnd('a2', '5'))
  assert.deepEqual(live(buildTimeline(ended, {}).items).at(-1), ['thinking', 'Thinking'], 'Claude is still to tell of their end')
  assert.deepEqual(live(buildTimeline(log(...ended, message('msg_2', 'Both are done.'), turnEnd()), {}).items), [], 'the work ends once it has')
})

test('no Thinking row after an interrupt', () => {
  const running = log(user('u1', 'Fix it'), toolUse('t1', 'Read'))
  assert.equal(buildTimeline(running, {}).items.at(-1)?.kind, 'thinking')
  const stopping = buildTimeline(log(...running, { kind: 'interrupt' }), {})
  assert.deepEqual([kinds(stopping.items), stopping.turnOpen], [['user', 'working', 'work'], true], 'it goes at once, before the turn ends')
  const next = buildTimeline(log(...running, { kind: 'interrupt' }, turnEnd(true), user('u2', 'Go on')), {})
  assert.equal(next.items.at(-1)?.kind, 'thinking', 'the next turn has it again')
})
