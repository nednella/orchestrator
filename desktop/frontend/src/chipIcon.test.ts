import assert from 'node:assert/strict'
import { test } from 'node:test'
import { chipIcon } from './chipIcon.ts'

test('a chip has the icon of its kind of file', () => {
  const table: [string, boolean, string][] = [
    ['src', true, 'folder'],
    ['a.ts', false, 'code'],
    ['main.go', false, 'code'],
    ['Makefile', false, 'code'],
    ['.env', false, 'code'],
    ['logo.PNG', false, 'image'],
    ['flow.svg', false, 'image'],
    ['README.md', false, 'text'],
    ['notes.txt', false, 'text'],
    ['page.html', false, 'web'],
    ['data.bin', false, 'file'],
    ['LICENSE', false, 'file'],
    ['archive.zip', false, 'file'],
  ]
  for (const [name, dir, want] of table) assert.equal(chipIcon(name, dir), want, name)
})
