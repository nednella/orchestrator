import assert from 'node:assert/strict'
import { test } from 'node:test'
import { getSchema } from '@tiptap/react'
import { EditorState, TextSelection } from '@tiptap/pm/state'
import { EMPTY_DRAFT, composerSchema, draftOf, fromMarkdown, fromPlainText, readDraft, saveDraft, toMarkdown } from './composerMarkdown.ts'

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
  table: '| a | b |\n| - | - |\n| 1 | 2 |',
  tableBetweenParagraphs: 'before\n\n| a | b |\n| - | - |\n| 1 | 2 |\n\nafter',
  tasks: '- [ ] task\n- [x] done',
  image: 'see ![img](x.png) here',
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

test('send value escapes a tilde unless it is the one ~ of a path', () => {
  assert.equal(toMarkdown(doc(p(text('move ~/a to ~/b')))), 'move \\~/a to \\~/b')
})

const lines = (value: string) => p(...value.split('\n').flatMap((line, i) => [...(i > 0 ? [{ type: 'hardBreak' }] : []), text(line)]))

const blockMarkers = {
  '>foo': '\\>foo',
  '# y': '\\# y',
  '- y': '\\- y',
  '+ y': '\\+ y',
  '* y': '\\* y',
  '1. y': '1\\. y',
  '2) y': '2\\) y',
  '---': '\\---',
  '- - -': '\\- - -',
  'a\n===': 'a\n\\===',
  'a\n-': 'a\n\\-',
  'a\n# y': 'a\n\\# y',
}

for (const [typed, expected] of Object.entries(blockMarkers)) {
  test(`send value escapes the block marker in a typed ${JSON.stringify(typed)}`, () => {
    const typedDoc = doc(lines(typed))
    assert.equal(toMarkdown(typedDoc), expected)
    assert.deepEqual(fromMarkdown(expected), typedDoc)
  })
}

test('send value escapes only the paragraph that needs it', () => {
  assert.equal(toMarkdown(doc(p(text('>foo')), p(text('use Vec<String> and snake_case')))), '\\>foo\n\nuse Vec<String> and snake_case')
})

test('send value never encodes <, > or &', () => {
  assert.equal(toMarkdown(doc(p(text('a < b & c > d, AT&T')))), 'a < b & c > d, AT&T')
  assert.equal(toMarkdown(doc(p(text('write &amp; it')))), 'write \\&amp; it')
})

test('a soft line break reads as a hard break', () => {
  assert.deepEqual(fromMarkdown('a\nb'), doc(p(text('a'), { type: 'hardBreak' }, text('b'))))
})

test('empty markdown reads as one empty paragraph', () => {
  assert.deepEqual(fromMarkdown(''), doc({ type: 'paragraph' }))
})

const unshown = {
  table: '| a | b |\n| - | - |\n| 1 | 2 |',
  image: '**bold** and ![img](x.png)',
  tasks: '- [ ] fix *this*\n- [x] read  the_issue',
  nestedTask: '- a\n\n  - [ ] t',
}

for (const [name, source] of Object.entries(unshown)) {
  test(`a loaded ${name} is literal and sent as its source, also after an edit elsewhere`, () => {
    const loaded = fromMarkdown(source)
    const literalOnly = loaded.content?.every((block) => block.content?.every((node) => node.marks?.some((m) => m.type === 'literal')))
    assert.equal(literalOnly, true)
    assert.equal(toMarkdown(loaded), source)
    const edited = doc(...(loaded.content ?? []), p(text('typed a*b')))
    assert.equal(toMarkdown(edited), `${source}\n\ntyped a\\*b`)
  })
}

const traceback = [
  'Traceback (most recent call last):',
  '  File "/app/pkg/__init__.py", line 3, in <module>',
  '    main()',
  '',
  '',
  '    # - not a list',
  'ValueError: bad *value*  ',
].join('\n')
const literal = (value: string) => text(value, 'literal')
const pasteBreak = { type: 'hardBreak', marks: [{ type: 'literal' }] }

test('pasted text loads as it is, not as markdown', () => {
  assert.deepEqual(fromPlainText('__init__.py\n    indented\r\n# not a heading'), doc(p(literal('__init__.py'), pasteBreak, literal('    indented'), pasteBreak, literal('# not a heading'))))
})

for (const pasted of ['    x = foo(a_b)\n\tindented\n__init__.py  ', traceback]) {
  test(`pasted text is sent exactly as pasted: ${JSON.stringify(pasted.slice(0, 20))}`, () => {
    assert.equal(toMarkdown(fromPlainText(pasted)), pasted)
  })
}

test('text typed right after pasted text is not literal', () => {
  const schema = getSchema(composerSchema)
  const pasted = EditorState.create({ doc: schema.nodeFromJSON(fromPlainText('a_b')) })
  const atEnd = pasted.apply(pasted.tr.setSelection(TextSelection.create(pasted.doc, pasted.doc.content.size - 1)))
  const typed = atEnd.apply(atEnd.tr.insertText(' *x* and # y'))
  assert.deepEqual(typed.doc.toJSON(), doc(p(literal('a_b'), text(' *x* and # y'))))
  assert.equal(toMarkdown(typed.doc.toJSON()), 'a_b \\*x\\* and # y')
})

test('typed text on the line after pasted text is escaped as a line start', () => {
  assert.equal(toMarkdown(doc(p(literal('foo'), pasteBreak, text('# x')))), 'foo\n\\# x')
})

test('a draft saved as a markdown string loads through the markdown reader', () => {
  const draft = readDraft('**hi** there\n\n- a')
  assert.deepEqual(draft.doc, fromMarkdown('**hi** there\n\n- a'))
  assert.equal(draft.text, '**hi** there\n\n- a')
})

test('a saved draft reloads as the same document', () => {
  const draft = draftOf(doc(p(text('run '), text('go test', 'code')), p(text('a*b'))))
  const reloaded = readDraft(JSON.parse(JSON.stringify(saveDraft(draft))))
  assert.deepEqual(reloaded, draft)
})

test('a saved draft that is not a document loads empty', () => {
  assert.equal(readDraft({ version: 1, doc: { type: 'doc', content: [{ type: 'nope' }] } }), EMPTY_DRAFT)
  assert.equal(readDraft({ version: 2, doc: doc(p(text('x'))) }), EMPTY_DRAFT)
  assert.equal(readDraft(null), EMPTY_DRAFT)
})
