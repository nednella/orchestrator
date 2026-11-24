import assert from 'node:assert/strict'
import { test } from 'node:test'
import { fromMarkdown, toMarkdown } from './composerMarkdown.ts'

const roundTrip = (text: string) => toMarkdown(fromMarkdown(text))

const samples = {
  marks: '**bold**, *italic*, ~~strike~~ and `code`',
  headings: '# One\n\n## Two\n\n### Three',
  bullets: '- a\n- b\n  - nested\n    - deeper\n- c',
  numbers: '1. one\n2. two\n   1. inner\n   2. inner two\n3. three',
  mixed: '1. step\n   - detail\n2. next',
  quote: '> quoted\n> still quoted',
  fence: '```go\nfunc main() {\n\tfmt.Println("hi")\n}\n```',
  fenceWithoutLanguage: '```\nplain block\n```',
  rule: 'above\n\n---\n\nbelow',
  link: 'see [the docs](https://example.com/docs) now',
  bareUrl: 'open https://example.com/a_b_c?x=1 please',
  literals: 'snake_case and 2 * 3 and a_b, ok? no: 4 * x',
  path: 'look in ~/code/agentos',
  newline: 'first line\nsecond line',
  paragraphs: 'one\n\ntwo',
  html: 'a Vec<String> and a < b and AT&T',
  issue: 'fix #123 then #45',
}

for (const [name, text] of Object.entries(samples)) {
  test(`round trip keeps ${name}`, () => {
    assert.equal(roundTrip(text), text)
  })
}

const normalised = {
  '__x__ and _y_': '**x** and *y*',
  '* a\n* b': '- a\n- b',
  '1) a\n2) b': '1. a\n2. b',
  '1. a\n1. b\n1. c': '1. a\n2. b\n3. c',
  '<https://example.com>': 'https://example.com',
  'hard  \nbreak': 'hard\nbreak',
  'Heading\n=======': '# Heading',
  '***': '---',
  'write &amp; it': 'write & it',
  'a\n\n\n\n\n\nb': 'a\n\n\n\nb',
}

for (const [text, expected] of Object.entries(normalised)) {
  test(`round trip normalises ${JSON.stringify(text)}`, () => {
    assert.equal(roundTrip(text), expected)
  })
}

const p = (...content: object[]) => ({ type: 'paragraph', content })
const text = (value: string, ...marks: string[]) => ({ type: 'text', text: value, ...(marks.length ? { marks: marks.map((type) => ({ type })) } : {}) })
const doc = (...content: object[]) => ({ type: 'doc', content })

test('send value writes a new line as one newline', () => {
  assert.equal(toMarkdown(doc(p(text('first'), { type: 'hardBreak' }, text('second')))), 'first\nsecond')
})

test('send value drops trailing empty paragraphs', () => {
  assert.equal(toMarkdown(doc(p(text('done')), p(), p())), 'done')
  assert.equal(toMarkdown(doc(p(text('a')), p(), p(), p(), p(text('b')))), 'a\n\n\n\nb')
  assert.equal(toMarkdown(doc(p())), '')
})

test('send value drops blank list items', () => {
  const item = (...content: object[]) => ({ type: 'listItem', content })
  assert.equal(toMarkdown(doc({ type: 'bulletList', content: [item(p(text('one'))), item(p())] })), '- one')
  assert.equal(toMarkdown(doc(p(text('a')), { type: 'bulletList', content: [item(p())] })), 'a')
})

test('send value escapes typed markup that would read as formatting', () => {
  assert.equal(toMarkdown(doc(p(text('a*b*c and `x` and __init__')))), 'a\\*b\\*c and \\`x\\` and \\_\\_init\\_\\_')
  assert.equal(roundTrip('a\\*b\\*c'), 'a\\*b\\*c')
})

test('send value keeps literal characters that cannot be formatting', () => {
  assert.equal(toMarkdown(doc(p(text('snake_case, 2 * 3, ~/code')))), 'snake_case, 2 * 3, ~/code')
})

test('send value writes styled text next to plain text', () => {
  assert.equal(toMarkdown(doc(p(text('run '), text('go test', 'code'), text(' then '), text('ship', 'bold')))), 'run `go test` then **ship**')
})

test('send value falls back to full escaping when the typed text would read differently', () => {
  assert.equal(toMarkdown(doc(p(text('move ~/a to ~/b')))), 'move \\~/a to \\~/b')
})

test('a soft line break reads as a hard break', () => {
  assert.deepEqual(fromMarkdown('a\nb'), doc(p(text('a'), { type: 'hardBreak' }, text('b'))))
})

test('empty markdown reads as one empty paragraph', () => {
  assert.deepEqual(fromMarkdown(''), doc({ type: 'paragraph' }))
})
