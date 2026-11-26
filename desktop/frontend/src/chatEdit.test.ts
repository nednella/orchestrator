import assert from 'node:assert/strict'
import { test } from 'node:test'
import { changesLabel, editBlocked, editRequest, laterMessages, undoRequest } from './chatEdit.ts'
import type { TimelineItem } from './chatTimeline.ts'

const idle = { turnOpen: false, pending: null, queued: [] }

test('Edit from here waits for the turn and the queue, and is off when Claude cannot rewind', () => {
  assert.equal(editBlocked('u1', '', idle), '')
  assert.equal(editBlocked('u1', '', { ...idle, turnOpen: true }), 'Edit from here when the turn ends')
  assert.equal(editBlocked('u1', '', { ...idle, pending: { id: 't', request: 'a1', tool: 'Bash', summary: '', kind: 'tool', input: null, inputTooLarge: false } }), 'Edit from here when the turn ends')
  assert.equal(editBlocked('u1', '', { ...idle, queued: [{ id: 'q1', text: 'Next', files: [] }] }), 'Edit from here once the queued messages are sent')
  assert.equal(editBlocked('', '', idle), "Claude's conversation does not hold this message as sent")
  const off = 'This version of Claude Code cannot rewind a thread'
  assert.equal(editBlocked('u1', off, idle), off)
  assert.equal(editBlocked('u1', off, { ...idle, turnOpen: true }), off, 'the reason for the run wins')
})

test('a dry run reads as its files and lines', () => {
  assert.equal(changesLabel({ files: ['/w/a.ts', '/w/b.ts', '/w/c.ts'], insertions: 12, deletions: 4 }), '3 files: +12 −4')
  assert.equal(changesLabel({ files: ['/w/a.ts'], insertions: 0, deletions: 1 }), '1 file: +0 −1')
})

const user = (uuid: string): TimelineItem => ({ kind: 'user', key: `user-${uuid}`, at: 0, id: uuid, uuid, text: uuid, files: [], forkAt: '' })
const reply = (uuid: string): TimelineItem => ({ kind: 'reply', key: `reply-${uuid}`, at: 0, id: uuid, uuid, text: uuid, streaming: false })

test('laterMessages counts the messages after the one rewound to', () => {
  const items: TimelineItem[] = [
    user('u1'),
    reply('r1'),
    user('u2'),
    { kind: 'work', key: 'w', startedAt: 0, endedAt: 1, calls: [], steps: [] },
    reply('r2'),
    { kind: 'notice', key: 'n', at: 0, text: 'Restarted', error: false },
    user('u3'),
    reply('r3'),
  ]
  assert.equal(laterMessages(items, 'u1'), 5)
  assert.equal(laterMessages(items, 'u3'), 1)
  assert.equal(laterMessages(items, 'gone'), 0)
})

test('Edit asks to rewind, says what goes and offers the files only in a worktree', () => {
  const r = editRequest(3, { offered: true, reason: '', files: [], insertions: 0, deletions: 0 })
  assert.equal(r.title, 'Edit from here?')
  assert.deepEqual(r.body, ['Rewinds the chat to before this message. Its text goes back in the composer, and you send it.', '3 later messages go.'])
  assert.deepEqual(r.checkbox, { label: "Also undo Claude's file changes since then", hint: 'Shows the files first', checked: false })
  assert.equal(r.confirm, 'Rewind')
  assert.equal(r.tone, 'primary')
  assert.deepEqual(editRequest(1, null).body?.[1], '1 later message goes.')
  assert.equal(editRequest(0, null).body?.length, 1)
  assert.equal(editRequest(1, null).checkbox, undefined, 'a thread in the project folder')
  assert.equal(editRequest(1, { offered: false, reason: '', files: [], insertions: 0, deletions: 0 }).checkbox, undefined)
})

test('the files box is off with the reason the offer gives', () => {
  const r = editRequest(2, { offered: true, reason: 'Thread #9 also works in this worktree', files: [], insertions: 0, deletions: 0 })
  assert.equal(r.checkbox?.disabled, 'Thread #9 also works in this worktree')
})

test('the files step lists the files and what undoing them changes', () => {
  const r = undoRequest({ files: ['/w/a.ts', '/w/b.ts'], insertions: 12, deletions: 4 }, ['a.ts', 'b.ts'])
  assert.equal(r.title, 'Undo these file changes?')
  assert.equal(r.body?.at(-1), '2 files: +12 −4')
  assert.deepEqual(r.list, ['a.ts', 'b.ts'])
  assert.equal(r.confirm, 'Rewind and undo')
  assert.equal(r.tone, 'danger')
})
