import assert from 'node:assert/strict'
import { test } from 'node:test'
import { shellsOf } from './shells.ts'
import type { Shell } from './types.ts'

const shell = (id: string, thread: string): Shell => ({ id, thread, command: 'zsh', busy: false })

const shells = [shell('main/shell', ''), shell('main/shell-2', 'main/aaaaaaaa'), shell('main/shell-3', 'main/bbbbbbbb'), shell('main/shell-4', 'main/aaaaaaaa')]

test("shellsOf keeps a thread's shells in order", () => {
  assert.deepEqual(shellsOf(shells, 'main/aaaaaaaa'), ['main/shell-2', 'main/shell-4'])
  assert.deepEqual(shellsOf(shells, 'main/cccccccc'), [])
})

test('project shells for no thread', () => {
  assert.deepEqual(shellsOf(shells, ''), ['main/shell'])
})
