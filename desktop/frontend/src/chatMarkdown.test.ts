import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import Markdown from 'react-markdown'
import { codeLanguage, lastLine, markdownProps, settled, textCache } from './chatMarkdown.ts'

const render = (children: string) => renderToStaticMarkup(createElement(Markdown, { ...markdownProps, children }))

test('a GFM table renders a table', () => {
  assert.equal(/<table>/.test(render('| a | b |\n| - | - |\n| 1 | 2 |')), true)
})

test('raw HTML is not rendered', () => {
  assert.equal(/<script|<b>/.test(render('hi <script>alert(1)</script> <b>x</b>')), false)
})

test('javascript links lose their href', () => {
  assert.equal(/javascript:/.test(render('[x](javascript:alert(1))')), false)
  assert.equal(/href="https:\/\/example.com"/.test(render('[x](https://example.com)')), true)
})

test('images are not loaded', () => {
  assert.equal(/<img/.test(render('![alt](https://example.com/a.png)')), false)
})

test('task list boxes are disabled', () => {
  const html = render('- [ ] todo\n- [x] done')
  assert.equal(/<input[^>]*disabled/.test(html), true)
  assert.equal(/<input(?![^>]*disabled)/.test(html), false)
})

test('codeLanguage reads language-go', () => {
  assert.equal(codeLanguage('language-go'), 'go')
  assert.equal(codeLanguage(undefined), '')
  assert.equal(codeLanguage('other'), '')
})

test('other links render with no usable href', () => {
  for (const url of ['JaVaScRiPt:alert(1)', 'data:text/html,hi', 'vbscript:x', '//evil.example', './file.md']) {
    const html = render(`[x](${url})`)
    assert.equal(/href="[^"]/.test(html), false, url)
  }
})

test('textCache lets the least recently used text go first', () => {
  const cache = textCache<number>(40)
  cache.set('a'.repeat(10), 1)
  cache.set('b'.repeat(10), 2)
  cache.set('c'.repeat(10), 3)
  assert.equal(cache.get('a'.repeat(10)), 1)
  cache.set('d'.repeat(10), 4)
  cache.set('e'.repeat(10), 5)
  assert.equal(cache.get('b'.repeat(10)), undefined)
  assert.deepEqual(['a', 'c', 'd', 'e'].map((c) => cache.get(c.repeat(10))), [1, 3, 4, 5])
})

test('textCache keeps no text over a quarter of its limit', () => {
  const cache = textCache<number>(40)
  cache.set('a'.repeat(11), 1)
  assert.equal(cache.get('a'.repeat(11)), undefined)
})

test('only what is above the line being written is whole', () => {
  assert.equal(lastLine('one\ntwo `src/a'), 2)
  assert.equal(lastLine('one\n'), 1)
  assert.equal(settled(0, 5), true, 'a block that is not being written')
  assert.equal(settled(2, 1), true)
  assert.equal(settled(2, 2), false)
  assert.equal(settled(2, undefined), false)
})
