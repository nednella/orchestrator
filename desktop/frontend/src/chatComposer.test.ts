import assert from 'node:assert/strict'
import { test } from 'node:test'
import { commandMatches, commandToken, composerAction, escapeAction, lastTurnEnd, mentionToken, quoteMarkdown, replyLabel, replyPrefix, sendText, stopHeld, unseenDrafts } from './chatComposer.ts'
import type { Command } from './chatTimeline.ts'
import type { ChatEvent, Pending } from './types.ts'

type Draft = Omit<ChatEvent, 'seq' | 'at'>

const log = (...drafts: Draft[]): ChatEvent[] => drafts.map((d, i) => ({ at: (i + 1) * 1000, ...d, seq: i + 1 }))
const draft = (text: string): Draft => ({ kind: 'draft', text })
const user = (id: string, text: string): Draft => ({ kind: 'user', id, text })

const waiting: Pending = { id: 'p/abc', request: 'req-1', tool: 'Bash', summary: 'npm install', kind: 'tool', input: { command: 'npm install' }, inputTooLarge: false }
const idle = { turnOpen: false, pending: null }
const running = { turnOpen: true, pending: null }
const approving = { turnOpen: true, pending: waiting }

const esc = { overlay: false, picker: false, turnOpen: true, stopping: false, queued: false }

test('escapeAction: an overlay closes first', () => {
  assert.equal(escapeAction({ ...esc, overlay: true, picker: true, queued: true }), 'close-overlay')
  assert.equal(escapeAction({ ...esc, overlay: true, turnOpen: false }), 'close-overlay')
})

test('escapeAction: a picker open closes the picker, before the turn is stopped', () => {
  assert.equal(escapeAction({ ...esc, picker: true }), 'close-picker')
  assert.equal(escapeAction({ ...esc, picker: true, queued: true }), 'close-picker')
  assert.equal(escapeAction({ ...esc, picker: true, turnOpen: false }), 'close-picker')
})

test('escapeAction: a running turn with no queued message is interrupted', () => {
  assert.equal(escapeAction(esc), 'interrupt')
})

test('escapeAction: a running turn with queued messages sends them now', () => {
  assert.equal(escapeAction({ ...esc, queued: true }), 'send-now')
})

test('escapeAction: a turn already stopping is not stopped again', () => {
  assert.equal(escapeAction({ ...esc, stopping: true }), 'none')
  assert.equal(escapeAction({ ...esc, stopping: true, queued: true }), 'none')
})

test('escapeAction: an idle thread blurs the composer', () => {
  assert.equal(escapeAction({ ...esc, turnOpen: false }), 'blur')
})

test('composerAction: an idle thread sends', () => {
  assert.equal(composerAction(idle, false), 'send')
  assert.equal(composerAction(idle, true), 'send')
})

test('composerAction: a running turn queues a message, and stops with an empty draft', () => {
  assert.equal(composerAction(running, false), 'queue')
  assert.equal(composerAction(running, true), 'stop')
})

test('composerAction: a message while an approval waits denies it', () => {
  assert.equal(composerAction(approving, false), 'deny')
  assert.equal(composerAction(approving, true), 'stop')
})

test('unseenDrafts: drafts after the last user message and the seen mark', () => {
  const events = log(user('u1', 'Hi'), draft('one'), draft('two'))
  assert.deepEqual(
    unseenDrafts(events, 0).map((e) => e.text),
    ['one', 'two'],
  )
  assert.deepEqual(
    unseenDrafts(events, 2).map((e) => e.text),
    ['two'],
  )
  assert.deepEqual(unseenDrafts(events, 3), [])
})

test('unseenDrafts: a draft followed by a user message is consumed', () => {
  assert.deepEqual(unseenDrafts(log(draft('Fix the checks.'), user('u1', 'Fix the checks.')), 0), [])
})

test('unseenDrafts: a resumed thread with no mark shows only the drafts after its last message', () => {
  const events = log(draft('old'), user('u1', 'old'), { kind: 'turn_end' }, { kind: 'exit' }, draft('new'))
  assert.deepEqual(
    unseenDrafts(events, 0).map((e) => e.text),
    ['new'],
  )
})

test('sendText keeps leading indent', () => {
  assert.equal(sendText('    indented code\n  \n\t'), '    indented code')
  assert.equal(sendText('  two\nlines  '), '  two\nlines')
})

test('lastTurnEnd is the seq of the latest turn end or exit', () => {
  assert.equal(lastTurnEnd(log(user('u1', 'Hi'))), 0)
  assert.equal(lastTurnEnd(log(user('u1', 'Hi'), { kind: 'turn_end' }, user('u2', 'Again'))), 2)
  assert.equal(lastTurnEnd(log(user('u1', 'Hi'), { kind: 'turn_end' }, { kind: 'exit' })), 3)
})

test('quote marks every line', () => {
  assert.equal(quoteMarkdown('one\ntwo\n\nthree\n'), '> one\n> two\n> \n> three\n\n')
})

test('reply prefix clips to three lines and 300 chars', () => {
  assert.equal(replyPrefix('a\nb\nc\nd'), '> a\n> b\n> c\n\n')
  const long = replyPrefix('x'.repeat(500))
  assert.equal(long, `> ${'x'.repeat(299)}…\n\n`)
})

test('empty selection quotes nothing', () => {
  assert.equal(quoteMarkdown(''), '')
  assert.equal(quoteMarkdown(' \n '), '')
  assert.equal(replyPrefix(''), '')
})

test('reply label is the first line, clipped to 80 chars', () => {
  assert.equal(replyLabel('\nfirst\nsecond'), 'first')
  assert.equal(replyLabel('y'.repeat(100)), `${'y'.repeat(79)}…`)
})

test('mentionToken: the path typed after an @ that starts a word', () => {
  assert.equal(mentionToken('@'), '')
  assert.equal(mentionToken('look at @src/app/main_test.go'), 'src/app/main_test.go')
  assert.equal(mentionToken('first line\n@docs/'), 'docs/')
  assert.equal(mentionToken('@../up-one.ts'), '../up-one.ts')
})

test('mentionToken: no mention inside a word or once a space ends it', () => {
  assert.equal(mentionToken('mail me@example'), null)
  assert.equal(mentionToken('@src/a.go '), null)
  assert.equal(mentionToken('no mention'), null)
})

test('slash only at the start', () => {
  assert.equal(commandToken('/'), '')
  assert.equal(commandToken('/rev'), 'rev')
  assert.equal(commandToken('/plugin:skill-name'), 'plugin:skill-name')
  assert.equal(commandToken('see /rev'), null)
  assert.equal(commandToken('first line\n/rev'), null)
  assert.equal(commandToken('/review the PR'), null)
})

test('prefix before contains', () => {
  const command = (name: string): Command => ({ name, description: '', argumentHint: '' })
  const commands = [command('pr-review'), command('compact'), command('Review'), command('init')]
  assert.deepEqual(
    commandMatches(commands, 'rev').map((c) => c.name),
    ['Review', 'pr-review'],
  )
  assert.deepEqual(
    commandMatches(commands, '').map((c) => c.name),
    ['pr-review', 'compact', 'Review', 'init'],
  )
})

test('stopHeld: a stop waits for the turn it stops, and a thread that starts holds none', () => {
  assert.equal(stopHeld({ turnOpen: true, starting: false, stopAsked: 4, lastEnd: 4 }), true)
  assert.equal(stopHeld({ turnOpen: true, starting: false, stopAsked: 4, lastEnd: 5 }), false, 'a turn ended since')
  assert.equal(stopHeld({ turnOpen: true, starting: false, stopAsked: null, lastEnd: 4 }), false)
  assert.equal(stopHeld({ turnOpen: true, starting: true, stopAsked: -1, lastEnd: -1 }), false, 'the cancel of a start is not waited for, so the retry can be stopped at once')
})
