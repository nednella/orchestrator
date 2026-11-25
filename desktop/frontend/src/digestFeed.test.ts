import assert from 'node:assert/strict'
import { test } from 'node:test'
import { filterCounts, groupByTool, isNewItem, itemDate, itemHost } from './digestFeed.ts'
import type { DigestItem } from './types'

const day = 86_400_000
const now = new Date(2026, 10, 25, 12, 0).getTime()
const item = (id: string, source: string, daysAgo: number, noteId = ''): DigestItem => ({
  id,
  title: `title ${id}`,
  why: '',
  url: `https://www.example.com/${id}`,
  source,
  at: now - daysAgo * day,
  noteId,
})

// As the digest keeps them: the latest run's items first.
const items = [
  item('b', 'Claude Code', 2),
  item('c', 'Wails ', 2),
  item('a', 'wails', 9),
  item('e', 'react', 9),
  item('d', 'Claude Code', 16, 'note-1'),
]
const seen = now - 5 * day

test('an item is new when it came after the last look and is not saved', () => {
  assert.equal(isNewItem(item('x', 'wails', 1), seen), true)
  assert.equal(isNewItem(item('x', 'wails', 1, 'note-2'), seen), false)
  assert.equal(isNewItem(item('x', 'wails', 6), seen), false)
})

test('the filters count every item, the new ones and the saved ones', () => {
  assert.deepEqual(filterCounts(items, seen), { all: 5, new: 2, saved: 1 })
  assert.deepEqual(filterCounts([], seen), { all: 0, new: 0, saved: 0 })
})

test('items group by tool, whatever case the run wrote the name in, in the order the digest keeps them', () => {
  const tools = groupByTool(items, 'all', seen)
  assert.deepEqual(
    tools.map((t) => [t.key, t.name, t.items.map((i) => i.id), t.fresh]),
    [
      ['claude code', 'Claude Code', ['b', 'd'], 1],
      ['wails', 'Wails', ['c', 'a'], 1],
      ['react', 'react', ['e'], 0],
    ],
  )
})

test('a filter keeps only the tools that have a matching item', () => {
  assert.deepEqual(
    groupByTool(items, 'new', seen).map((t) => [t.name, t.items.map((i) => i.id)]),
    [
      ['Claude Code', ['b']],
      ['Wails', ['c']],
    ],
  )
  assert.deepEqual(
    groupByTool(items, 'saved', seen).map((t) => [t.name, t.items.map((i) => i.id)]),
    [['Claude Code', ['d']]],
  )
})

test('an item with no tool goes under Other', () => {
  assert.deepEqual(
    groupByTool([item('a', '  ', 1)], 'all', seen).map((t) => t.name),
    ['Other'],
  )
})

test('the host drops www and is empty for a link that does not parse', () => {
  assert.equal(itemHost('https://www.example.com/x'), 'example.com')
  assert.equal(itemHost('https://code.claude.com/docs/changelog'), 'code.claude.com')
  assert.equal(itemHost('not a link'), '')
})

test('an item date has its year only when it is not this year', () => {
  assert.equal(itemDate(new Date(2026, 10, 23).getTime(), now), '23 Nov')
  assert.equal(itemDate(new Date(2025, 11, 30).getTime(), now), '30 Dec 2025')
})
