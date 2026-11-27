import assert from 'node:assert/strict'
import { test } from 'node:test'
import { imagePath, isWidePreview, previewCSP, previewDocument, previewFrame, previewParts } from './questionPreview.ts'

test('plain text is a code block, markdown is markdown', () => {
  assert.deepEqual(previewParts('+------+\n| Cart |\n+------+'), [{ kind: 'code', text: '+------+\n| Cart |\n+------+' }])
  assert.deepEqual(previewParts('- a\n- b'), [{ kind: 'code', text: '- a\n- b' }], 'a dash alone may be a mockup')
  assert.deepEqual(previewParts('## Stepper\n\nThree **routes**'), [{ kind: 'markdown', text: '## Stepper\n\nThree **routes**' }])
  assert.deepEqual(previewParts('```ts\nconst a = 1\n```'), [{ kind: 'markdown', text: '```ts\nconst a = 1\n```' }])
  assert.deepEqual(previewParts(''), [])
  assert.deepEqual(previewParts('  \n\n'), [])
})

test('images and html blocks come out in order', () => {
  const parts = previewParts('Before\n\n![Stepper](/tmp/a.png)\n\n```html\n<b>hi</b>\n```\n\n![](shots/b.png "title")')
  assert.deepEqual(parts, [
    { kind: 'code', text: 'Before' },
    { kind: 'image', alt: 'Stepper', src: '/tmp/a.png' },
    { kind: 'html', html: '<b>hi</b>' },
    { kind: 'image', alt: '', src: 'shots/b.png' },
  ])
})

test('a preview that is only an image or a page takes a whole row, and so does a wide line', () => {
  assert.equal(isWidePreview(previewParts('![a](/tmp/a.png)')), true)
  assert.equal(isWidePreview(previewParts('```html\n<p>x</p>\n```')), true)
  assert.equal(isWidePreview(previewParts('/checkout/address\n/checkout/payment')), false)
  assert.equal(isWidePreview(previewParts('x'.repeat(60))), true)
  assert.equal(isWidePreview([]), false)
})

test('an image is a path, never an address', () => {
  assert.equal(imagePath('/tmp/a.png'), '/tmp/a.png')
  assert.equal(imagePath('shots/a.png'), 'shots/a.png')
  assert.equal(imagePath('file:///tmp/a%20b.png'), '/tmp/a b.png')
  for (const src of ['https://example.com/a.png', 'http://x/a.png', 'data:image/png;base64,AAAA', 'javascript:alert(1)', 'ftp://x/a.png']) assert.equal(imagePath(src), null, src)
})

test('the page of an html block carries the policy first and the theme tokens, and the sandbox is empty', () => {
  const values = { 'color-scheme': 'dark', '--bg-app': '#000', '--text': '#f5f5f5', '--accent': 'oklch(0.571 0.21 264)', '--danger': '</style><script>x</script>', '--ignored': '1' }
  const { srcDoc, sandbox } = previewFrame('<h1 style="color:var(--accent)">Hi</h1>', values)
  assert.equal(sandbox, '')
  assert.equal(srcDoc.startsWith(`<meta http-equiv="Content-Security-Policy" content="${previewCSP}">`), true, srcDoc)
  assert.equal(previewCSP, "default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:")
  assert.equal(srcDoc.includes(':root{color-scheme:dark;--bg-app:#000;--text:#f5f5f5;--accent:oklch(0.571 0.21 264)}'), true, srcDoc)
  assert.equal(!srcDoc.includes('--ignored') && !srcDoc.includes('<script'), true, 'a value that would end the rule is left out')
  assert.equal(srcDoc.endsWith('<h1 style="color:var(--accent)">Hi</h1>'), true)
  assert.equal(previewDocument('', {}), `<meta http-equiv="Content-Security-Policy" content="${previewCSP}"><style>:root{}</style>`)
})
