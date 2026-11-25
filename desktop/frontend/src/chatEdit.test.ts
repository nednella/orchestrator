import assert from 'node:assert/strict'
import { test } from 'node:test'
import { changesLabel, editBlocked } from './chatEdit.ts'

const idle = { turnOpen: false, pending: null, queued: [] }

test('Edit from here waits for the turn and the queue, and is off when Claude cannot rewind', () => {
  assert.equal(editBlocked('u1', '', idle), '')
  assert.equal(editBlocked('u1', '', { ...idle, turnOpen: true }), 'Edit from here when the turn ends')
  assert.equal(editBlocked('u1', '', { ...idle, pending: { id: 't', request: 'a1', tool: 'Bash', summary: '', kind: 'tool', input: null } }), 'Edit from here when the turn ends')
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
