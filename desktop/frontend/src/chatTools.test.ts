import assert from 'node:assert/strict'
import { test } from 'node:test'
import { formatWorked, resultLines, toolBody, toolSummary } from './chatTools.ts'

const folder = '/work/shop'

test('Bash shows its description and the first line of the command', () => {
  const s = toolSummary('Bash', { command: 'npm test\nnpm run build', description: 'Run the tests' }, folder)
  assert.deepEqual([s.title, s.detail], ['Run the tests', 'npm test'])
  assert.equal(toolSummary('Bash', { command: 'ls' }, folder).title, 'Run')
})

test('Read shows its path', () => {
  const s = toolSummary('Read', { file_path: '/work/shop/src/a.ts' }, folder)
  assert.deepEqual([s.title, s.detail], ['Read', 'src/a.ts'])
})

test('Edit, MultiEdit and Write count their lines', () => {
  const edit = toolSummary('Edit', { file_path: '/work/shop/a.ts', old_string: 'a\nb', new_string: 'c\nd\ne' }, folder)
  assert.deepEqual([edit.title, edit.detail], ['Edit', 'a.ts +3 −2'])
  const multi = toolSummary('MultiEdit', { file_path: '/work/shop/a.ts', edits: [{ old_string: 'a', new_string: 'b' }, { old_string: 'c', new_string: '' }] }, folder)
  assert.equal(multi.detail, 'a.ts +1 −2')
  const write = toolSummary('Write', { file_path: '/work/shop/a.ts', content: 'x\ny\nz' }, folder)
  assert.deepEqual([write.title, write.detail], ['Write', 'a.ts +3 −0'])
})

test('an Edit that replaces every match says so', () => {
  assert.equal(toolSummary('Edit', { file_path: 'a.ts', old_string: 'a', new_string: 'b', replace_all: true }, folder).detail, 'a.ts +1 −1 · all')
})

test('Grep and Glob show the pattern', () => {
  assert.deepEqual(toolSummary('Grep', { pattern: 'foo.*' }, folder), { icon: 'search', title: 'Grep', detail: 'foo.*' })
  assert.equal(toolSummary('Glob', { pattern: '**/*.ts' }, folder).detail, '**/*.ts')
})

test('WebFetch shows the URL and WebSearch the query', () => {
  assert.equal(toolSummary('WebFetch', { url: 'https://example.com' }, folder).detail, 'https://example.com')
  assert.equal(toolSummary('WebSearch', { query: 'go slog' }, folder).detail, 'go slog')
})

test('Task and Agent show their description', () => {
  assert.equal(toolSummary('Task', { description: 'Find the router' }, folder).detail, 'Find the router')
  assert.equal(toolSummary('Agent', { description: 'Review' }, folder).detail, 'Review')
})

test('TodoWrite shows how many tasks', () => {
  assert.equal(toolSummary('TodoWrite', { todos: [{}, {}, {}] }, folder).detail, '3 tasks')
  assert.equal(toolSummary('TodoWrite', { todos: [{}] }, folder).detail, '1 task')
})

test('an MCP tool shows its server and name', () => {
  const s = toolSummary('mcp__github__create_issue', { title: 'x' }, folder)
  assert.deepEqual([s.title, s.detail], ['github · create_issue', ''])
})

test('any other tool shows its input as JSON, clipped to 120 characters', () => {
  assert.equal(toolSummary('Weird', { a: 1 }, folder).detail, '{"a":1}')
  const long = toolSummary('Weird', { text: 'x'.repeat(300) }, folder).detail
  assert.equal(long.length, 120)
  assert.equal(long.endsWith('…'), true)
})

test('a missing or odd input does not throw', () => {
  assert.equal(toolSummary('Read', null, folder).detail, '')
  assert.equal(toolSummary('Bash', 'text', folder).title, 'Run')
})

test('paths are shown relative to the thread folder', () => {
  assert.equal(toolSummary('Read', { file_path: '/work/shop/a/b.ts' }, folder).detail, 'a/b.ts')
  assert.equal(toolSummary('Read', { file_path: '/work/shop/a/b.ts' }, '/work/shop/').detail, 'a/b.ts')
  assert.equal(toolSummary('Read', { file_path: '/work/shop-other/b.ts' }, folder).detail, '/work/shop-other/b.ts')
  assert.equal(toolSummary('Read', { file_path: '/etc/hosts' }, folder).detail, '/etc/hosts')
  assert.equal(toolSummary('Read', { file_path: '/etc/hosts' }, '').detail, '/etc/hosts')
})

test('formatWorked', () => {
  assert.equal(formatWorked(4000), '4s')
  assert.equal(formatWorked(59_999), '59s')
  assert.equal(formatWorked(60_000), '1m')
  assert.equal(formatWorked(134_000), '2m 14s')
  assert.equal(formatWorked(3_780_000), '1h 3m')
  assert.equal(formatWorked(3_723_000), '1h 2m 3s')
  assert.equal(formatWorked(3_603_000), '1h 3s')
  assert.equal(formatWorked(3_600_000), '1h')
  assert.equal(formatWorked(-5), '0s')
  assert.equal(formatWorked(420), '0s')
})

test('a finished turn\'s time is precise below ten seconds', () => {
  assert.deepEqual([420, 6_340, 9_990, 10_000, 42_500, 63_000].map((ms) => formatWorked(ms, true)), ['420ms', '6.3s', '9.9s', '10s', '42s', '1m 3s'])
})

test('an Edit opens as a diff and a Bash as its full command', () => {
  assert.deepEqual(toolBody('Edit', { file_path: 'a', old_string: 'x\ny', new_string: 'z' }), {
    diff: [
      { sign: '−', text: 'x' },
      { sign: '−', text: 'y' },
      { sign: '+', text: 'z' },
    ],
  })
  assert.deepEqual(toolBody('Bash', { command: 'a\nb' }), { text: 'a\nb' })
  assert.deepEqual(toolBody('Write', { file_path: 'a', content: 'q' }), { diff: [{ sign: '+', text: 'q' }] })
  assert.deepEqual(toolBody('Grep', { pattern: 'p' }), { text: '{\n  "pattern": "p"\n}' })
})

test('the edits of a MultiEdit are told apart', () => {
  const body = toolBody('MultiEdit', { file_path: 'a', edits: [{ old_string: 'a', new_string: 'b' }, { old_string: 'c', new_string: 'd' }] })
  assert.deepEqual(body, {
    diff: [
      { sign: '−', text: 'a' },
      { sign: '+', text: 'b' },
      { sign: ' ', text: '…' },
      { sign: '−', text: 'c' },
      { sign: '+', text: 'd' },
    ],
  })
})

test('a result shows its first 20 lines until all are asked for', () => {
  const text = Array.from({ length: 25 }, (_, i) => `l${i + 1}`).join('\n')
  const clipped = resultLines(text, false)
  assert.equal(clipped.total, 25)
  assert.equal(clipped.shown.split('\n').length, 20)
  assert.equal(resultLines(text, true).shown, text)
  assert.equal(resultLines('one', false).total, 1)
})
