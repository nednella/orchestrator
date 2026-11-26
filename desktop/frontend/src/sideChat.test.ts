import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ownStart, sideChatAsked, sideChatCommands, sideChatDot, sideChatQuestion, sideChatReason } from './sideChat.ts'
import type { ChatEvent, Session } from './types.ts'

type Draft = Omit<ChatEvent, 'seq' | 'at'>

const log = (...drafts: Draft[]): ChatEvent[] => drafts.map((d, i) => ({ at: (i + 1) * 1000, ...d, seq: i + 1 }))
const user = (id: string, text = 'ask'): Draft => ({ kind: 'user', id, text })
const reply = (uuid: string, agent?: string): Draft => ({ kind: 'message', id: `msg-${uuid}`, uuid, text: 'answer', ...(agent ? { agent } : {}) })
const end: Draft = { kind: 'turn_end' }
const thread = (over: Partial<Session> = {}) => ({ id: 'shop/abcd2345', parent: '', side: false, ...over }) as Session

test('/side and /btw take the rest of the message as the question', () => {
  assert.equal(sideChatQuestion('/side why is the cart slow?'), 'why is the cart slow?')
  assert.equal(sideChatQuestion('/btw  what about\nthe tests? '), 'what about\nthe tests?')
  assert.equal(sideChatQuestion('/SIDE'), '', 'no question opens an empty side chat')
  assert.equal(sideChatQuestion(' /btw '), '')
  for (const text of ['/sides', '/review', 'side note', 'ask /side', '']) assert.equal(sideChatQuestion(text), null, text)
})

test('the slash menu shows why a side chat cannot start in place of what the command does', () => {
  assert.deepEqual(
    sideChatCommands('').map((c) => [c.name, c.description]),
    [['side', 'Ask a side question in a new side chat'], ['btw', 'Same as /side']],
  )
  assert.deepEqual(
    sideChatCommands('Side chats start after the first message.').map((c) => c.description),
    ['Side chats start after the first message.', 'Side chats start after the first message.'],
  )
})

test('a side chat starts from a finished reply of the thread', () => {
  assert.equal(sideChatReason(undefined), 'Available from a thread.')
  assert.equal(sideChatReason(thread({ side: true })), 'Side chats cannot start side chats.')
  assert.equal(sideChatReason(thread({ parent: 'shop/lead2345' })), 'Subagent threads cannot start side chats.')
  assert.equal(sideChatReason(thread()), '', 'a chat not loaded yet is left to the sessions')
  assert.equal(sideChatReason(thread(), log({ kind: 'start' }, user('u1'))), 'Side chats start after the first message.')
  assert.equal(sideChatReason(thread(), log(user('u1'), reply('r1'))), 'Side chats start after the first message.', 'a reply that still streams')
  assert.equal(sideChatReason(thread(), log(user('u1'), reply('r1', 'call'), end)), 'Side chats start after the first message.', "a subagent's reply")
  assert.equal(sideChatReason(thread(), log(user('u1'), reply('r1'), end, user('u2'), reply('r2'))), '', 'while the next turn runs')
  assert.equal(sideChatReason(thread(), log(user('u1'), reply('r1'), end, { kind: 'rewind', id: 'u1' })), 'Side chats start after the first message.', 'a rewind takes the reply back')
  assert.equal(sideChatReason(thread(), log(user('u1'), reply('r1'), end, { kind: 'compact' }, user('u2'))), "This thread's last reply is from before its chat was compacted. Side chats start after its next reply.")
  assert.equal(sideChatReason(thread(), log(user('u1'), reply('r1'), end, { kind: 'compact' }, user('u2'), reply('r2'), end)), '')
})

test("a side chat's own chat starts at its host's first start", () => {
  const forked = log(user('u1'), reply('r1'), end)
  assert.equal(ownStart(forked), 3, "before the start, all of it is the thread's")
  assert.equal(sideChatAsked(forked), false)
  const started = log(user('u1'), reply('r1'), end, { kind: 'start' }, { kind: 'commands' })
  assert.equal(ownStart(started), 3)
  assert.equal(sideChatAsked(started), false)
  assert.equal(sideChatAsked(log(user('u1'), reply('r1'), end, { kind: 'start' }, user('u2'))), true)
  assert.equal(sideChatAsked(log({ kind: 'start' }, { kind: 'user', id: 'u2', text: 'deny', parent: 'req-1' })), false, 'the reason of a denial is no message of its own')
})

test("a side chat's tab dot shows while it works or needs the owner", () => {
  assert.equal(sideChatDot('working'), 'busy')
  assert.equal(sideChatDot('waiting'), 'asks')
  assert.equal(sideChatDot('idle'), null)
  assert.equal(sideChatDot('settled'), null)
})
