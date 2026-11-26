import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import Markdown from 'react-markdown'
import { markdownProps } from './chatMarkdown.ts'
import { markdownBlocks, steadyBlock } from './markdownBlocks.ts'

const render = (children: string) => renderToStaticMarkup(createElement(Markdown, { ...markdownProps, children }))

// The blocks render as the whole text does, but for the line breaks between top-level elements.
const sameAsWhole = (text: string) => {
  const blocks = markdownBlocks(text)
  const joined = (html: string) => html.replace(/>\n</g, '><')
  assert.equal(joined(blocks.map(render).join('')), joined(render(text)), JSON.stringify(blocks))
}

test('paragraphs and headings are cut at blank lines', () => {
  assert.deepEqual(markdownBlocks('## Title\n\nOne.\n\n\nTwo.'), ['## Title', 'One.', 'Two.'])
})

test('a fenced code block is never cut', () => {
  assert.deepEqual(markdownBlocks('```ts\na\n\nb\n```\n\nc'), ['```ts\na\n\nb\n```', 'c'])
  assert.deepEqual(markdownBlocks('~~~~\na\n\n~~~\nb\n~~~~\n\nc'), ['~~~~\na\n\n~~~\nb\n~~~~', 'c'])
  assert.deepEqual(markdownBlocks('```\na\n\nb'), ['```\na\n\nb'])
})

test('a list is never cut, with its items or indented continuations after blank lines', () => {
  assert.deepEqual(markdownBlocks('- a\n\n- b\n\nAfter.'), ['- a\n\n- b', 'After.'])
  assert.deepEqual(markdownBlocks('1. a\n\n2. b\n\n   more of b\n\nAfter.'), ['1. a\n\n2. b\n\n   more of b', 'After.'])
  assert.deepEqual(markdownBlocks('Intro:\n- a\n\n- b'), ['Intro:\n- a\n\n- b'])
})

test('a line still being written after a list is not cut while it may become an item', () => {
  assert.deepEqual(markdownBlocks('- a\n\n-'), ['- a\n\n-'])
  assert.deepEqual(markdownBlocks('- a\n\n12'), ['- a\n\n12'])
  assert.deepEqual(markdownBlocks('- a\n\nN'), ['- a', 'N'])
  assert.deepEqual(markdownBlocks('- a\n\n12 more'), ['- a', '12 more'])
})

test('a code fence inside a list item ends with the item', () => {
  assert.deepEqual(markdownBlocks('- a\n  ```\n  x\n\ny\n\nz'), ['- a\n  ```\n  x', 'y', 'z'])
  assert.deepEqual(markdownBlocks('- a\n\n  ```\n  x\n\n  y\n  ```\n\nz'), ['- a\n\n  ```\n  x\n\n  y\n  ```', 'z'])
})

test('an indented code block is not cut', () => {
  assert.deepEqual(markdownBlocks('Code:\n\n    a\n\n    b\n\nAfter.'), ['Code:\n\n    a\n\n    b', 'After.'])
})

test('tables and block quotes are cut only after they end', () => {
  assert.deepEqual(markdownBlocks('| a | b |\n| - | - |\n| 1 | 2 |\n\nAfter.'), ['| a | b |\n| - | - |\n| 1 | 2 |', 'After.'])
  assert.deepEqual(markdownBlocks('> a\n>\n> b\n\nAfter.'), ['> a\n>\n> b', 'After.'])
})

test('a text with link reference definitions, footnotes or raw HTML is one block', () => {
  for (const text of ['See [the docs][d].\n\n[d]: https://example.com', 'See [x][a].\n\n> [a]: https://example.com', 'See [x][a].\n\n- [a]: https://example.com', 'A claim.[^1]\n\nMore.\n\n[^1]: The source.', '<div>\n\nhi\n\n</div>\n\nAfter.']) {
    assert.deepEqual(markdownBlocks(text), [text])
  }
})

test('markup inside a code fence does not tie blocks together', () => {
  assert.deepEqual(markdownBlocks('```html\n<div>\n[a]: b\n```\n\nAfter.'), ['```html\n<div>\n[a]: b\n```', 'After.'])
})

test('CRLF line ends are cut as LF ones', () => {
  assert.deepEqual(markdownBlocks('Intro\r\n\r\n```ts\r\na\r\n\r\nb\r\n```\r\n\r\n- a\r\n\r\n- b\r\n\r\nEnd.'), ['Intro\r', '```ts\r\na\r\n\r\nb\r\n```\r', '- a\r\n\r\n- b\r', 'End.'])
})

test('a fence closes only at a line indented at most 3 spaces', () => {
  assert.deepEqual(markdownBlocks('  ```\na\n     ```\n\nb\n  ```\n\nc'), ['  ```\na\n     ```\n\nb\n  ```', 'c'])
})

test('a heading ends a list, so a fence after it is not the list item\'s', () => {
  assert.deepEqual(markdownBlocks('- a\n# h\n  ```\nx\n\ny\n  ```\n\nz'), ['- a\n# h\n  ```\nx\n\ny\n  ```', 'z'])
  assert.deepEqual(markdownBlocks('- a\n# h\n\n- b'), ['- a\n# h', '- b'])
})

const texts = [
  '## Title\n\nSome **bold** and `code`.\n\n- a\n\n- b\n  - nested\n\n1. one\n2. two\n\nAfter.',
  '```ts\nconst a = 1\n\nconst b = 2\n```\n\n| a | b |\n| - | -: |\n| 1 | 2 |\n\n> quoted\n>\n> more\n\n---\n\nEnd.',
  '- a\n  ```\n  x\n\ny\n\n    indented\n\n    code\n\nLast.',
  'Para\nnext line\n\n* * *\n\n- [ ] todo\n- [x] done\n\n~~~\n~~~',
  'Intro\r\n\r\n```ts\r\nconst a = 1\r\n\r\nconst b = 2\r\n```\r\n\r\n- a\r\n\r\n- b\r\n  more\r\n\r\n1. one\r\n2. two\r\n\r\nEnd.',
  '  ```\na\n     ```\n\nb\n  ```\n\nc',
  '- a\n# h\n  ```\nx\n\ny\n  ```\n\n- a\n# h\n\n- b',
]

test('the blocks render as the whole text', () => {
  texts.forEach(sameAsWhole)
})

test('a block once closed stays the same as the text grows', () => {
  const text = texts.join('\n\n')
  const whole = markdownBlocks(text)
  for (let n = 1; n < text.length; n++) {
    const blocks = markdownBlocks(text.slice(0, n))
    blocks.slice(0, -1).forEach((block, i) => assert.equal(block, whole[i], `at ${n}`))
  }
})

test('the block being written closes its inline markers', () => {
  assert.equal(steadyBlock('Some **bo'), 'Some **bo**')
  assert.equal(steadyBlock('Some *it'), 'Some *it*')
  assert.equal(steadyBlock('Run `npm t'), 'Run `npm t`')
  assert.equal(steadyBlock('***both'), '***both***')
  assert.equal(steadyBlock('**a *b '), '**a *b***')
  assert.equal(steadyBlock('**done** and *x'), '**done** and *x*')
  assert.equal(steadyBlock('- the **sub'), '- the **sub**')
  assert.equal(steadyBlock('**see `foo'), '**see `foo`**')
})

test('the block being written drops markers with nothing after them yet', () => {
  assert.equal(steadyBlock('Some **'), 'Some ')
  assert.equal(steadyBlock('Some *'), 'Some ')
  assert.equal(steadyBlock('Run `'), 'Run ')
  assert.equal(steadyBlock('**`'), '')
})

test('the block being written leaves a star or underscore inside a word alone', () => {
  assert.equal(steadyBlock('see src/*.ts and my_var'), 'see src/*.ts and my_var')
  assert.equal(steadyBlock('Read src/*.ts **now'), 'Read src/*.ts **now**')
})

test('the block being written keeps CRLF line ends', () => {
  assert.equal(steadyBlock('```ts\r\nconst a = 1\r\nconst b'), '```ts\r\nconst a = 1\r\n')
  assert.equal(steadyBlock('| a | b |\r\n| --- | --- |\r\n'), '| a | b |\r\n| --- | --- |\r\n')
  assert.equal(steadyBlock('Text\r\nSome **bo'), 'Text\r\nSome **bo**')
})

test('the block being written leaves plain stars alone', () => {
  assert.equal(steadyBlock('2 * 3 = 6'), '2 * 3 = 6')
  assert.equal(steadyBlock('a \\*b'), 'a \\*b')
  assert.equal(steadyBlock('**done**'), '**done**')
})

test('the block being written holds back a line of block markers', () => {
  assert.equal(steadyBlock('Text\n##'), 'Text\n')
  assert.equal(steadyBlock('- a\n- '), '- a\n')
  assert.equal(steadyBlock('Text\n1.'), 'Text\n')
  assert.equal(steadyBlock('Text\n--'), 'Text\n')
  assert.equal(steadyBlock('Text\n```t'), 'Text\n')
  assert.equal(steadyBlock('Text\n1. R'), 'Text\n1. R')
})

test('the block being written shows only whole lines of an open code fence', () => {
  assert.equal(steadyBlock('```ts\nconst a = 1\nconst b'), '```ts\nconst a = 1\n')
  assert.equal(steadyBlock('```ts\nconst a = 1\n``'), '```ts\nconst a = 1\n')
  assert.equal(steadyBlock('```ts\nconst a = 1\n```\nAfter **x'), '```ts\nconst a = 1\n```\nAfter **x**')
})

test('the block being written holds back a table until its delimiter row is whole', () => {
  assert.equal(steadyBlock('| a | b |'), '')
  assert.equal(steadyBlock('| a | b |\n'), '')
  assert.equal(steadyBlock('| a | b |\n| --'), '')
  assert.equal(steadyBlock('| a | b |\n| --- | -'), '')
  assert.equal(steadyBlock('| a | b |\n| --- | --- |\n'), '| a | b |\n| --- | --- |\n')
  assert.equal(steadyBlock('| a | b |\n| --- |\n'), '')
  assert.equal(steadyBlock('| a | b |\n| --- | --- |\n| 1 | **2'), '| a | b |\n| --- | --- |\n| 1 | **2**')
  assert.equal(steadyBlock('Intro\n| a | b |\n| -'), 'Intro\n')
})

test('a long line that matches no rule is read in linear time', () => {
  // A cutter that backtracks takes quadratic time: a hundred thousand characters take minutes, where a linear one takes
  // milliseconds. The bound is two seconds, three orders above a linear run, so no load on the machine can fail it and
  // no ratio of two timings is read.
  const LIMIT_MS = 2000
  const linear = (name: string, run: (n: number) => void) => {
    const start = performance.now()
    run(100_000)
    const took = performance.now() - start
    assert.equal(took < LIMIT_MS, true, `${name} took ${Math.round(took)} ms for 100k characters`)
  }
  linear('footnote-like', (n) => markdownBlocks('[^'.repeat(n / 2)))
  linear('markers then a letter', (n) => steadyBlock(`${' '.repeat(n)}x`))
  linear('rule then a letter', (n) => markdownBlocks(`---${' '.repeat(n)}x`))
  linear('unclosed bold', (n) => steadyBlock('**a '.repeat(n / 4)))
})
