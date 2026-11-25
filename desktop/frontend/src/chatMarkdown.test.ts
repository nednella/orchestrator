import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import Markdown from 'react-markdown'
import { codeLanguage, markdownProps } from './chatMarkdown.ts'

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
