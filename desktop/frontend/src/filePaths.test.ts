import assert from 'node:assert/strict'
import { test } from 'node:test'
import { chipNames, codeRef, linkRef, placeLabel, refText } from './filePaths.ts'
import type { FileRef } from './filePaths.ts'

const link = (href: string) => linkRef(href)
const code = (text: string) => codeRef(text)

test('a link to a path is a file ref', () => {
  const table: [string, FileRef][] = [
    ['/private/tmp/claude-501/scratchpad/rule-inventory.html', { path: '/private/tmp/claude-501/scratchpad/rule-inventory.html' }],
    ['/Users/me/code/app/src/a.ts', { path: '/Users/me/code/app/src/a.ts' }],
    ['./src/a.ts', { path: './src/a.ts' }],
    ['../docs/plan.md', { path: '../docs/plan.md' }],
    ['~/notes/todo.md', { path: '~/notes/todo.md' }],
    ['README.md', { path: 'README.md' }],
    ['src/lib', { path: 'src/lib' }],
    ['src/lib/', { path: 'src/lib' }],
    ['Makefile', { path: 'Makefile' }],
    ['Dockerfile', { path: 'Dockerfile' }],
    ['docs/My%20Plan.md', { path: 'docs/My Plan.md' }],
    ['/tmp/dir/', { path: '/tmp/dir' }],
    ['file:///tmp/a%20b/page.html', { path: '/tmp/a b/page.html' }],
    ['file://localhost/tmp/page.html', { path: '/tmp/page.html' }],
    ['docs/plan.md#heading', { path: 'docs/plan.md' }],
    ['docs/plan.md?raw=1', { path: 'docs/plan.md' }],
    ['a.ts:12', { path: 'a.ts', line: 12 }],
  ]
  for (const [href, want] of table) assert.deepEqual(link(href), want, href)
})

test('a link that is not a local path is no file ref', () => {
  const table = [
    '',
    '#heading',
    '//example.com/a.png',
    'http://example.com/a.png',
    'https://example.com',
    'mailto:me@example.com',
    'tel:+4912345',
    'javascript:alert(1)',
    'ftp://example.com/a.txt',
    'vscode://file/a.ts',
    'file://example.com/etc/passwd',
    'foo',
    'example.com',
    'example.com/readme.md',
    'localhost:3000',
    'localhost/a.html',
    '127.0.0.1/a.html',
    '1.2.3',
    '/private/tmp/…/page.html',
    '/private/tmp/.../page.html',
    'src/.../a.ts',
    '…/a.ts',
    '.',
    '..',
    '../',
    '/',
    '~',
    'http:80',
  ]
  for (const href of table) assert.equal(link(href), null, href)
})

test('a link names a place in the file by a suffix or a fragment', () => {
  const table: [string, FileRef][] = [
    ['src/a.ts:42', { path: 'src/a.ts', line: 42 }],
    ['src/a.ts:42:7', { path: 'src/a.ts', line: 42, col: 7 }],
    ['src/a.ts:42-64', { path: 'src/a.ts', line: 42, endLine: 64 }],
    ['src/a.ts:42–64', { path: 'src/a.ts', line: 42, endLine: 64 }],
    ['src/a.ts#L42', { path: 'src/a.ts', line: 42 }],
    ['src/a.ts#L42C7', { path: 'src/a.ts', line: 42, col: 7 }],
    ['src/a.ts#L42-L64', { path: 'src/a.ts', line: 42, endLine: 64 }],
    ['src/a.ts#L42-64', { path: 'src/a.ts', line: 42, endLine: 64 }],
    ['file:///tmp/a.ts#L9', { path: '/tmp/a.ts', line: 9 }],
    ['src/a.ts:5#L99', { path: 'src/a.ts', line: 5 }],
    ['src/a.ts:0', { path: 'src/a.ts' }],
    ['src/a.ts:0:9', { path: 'src/a.ts' }],
    ['src/a.ts:3:0', { path: 'src/a.ts', line: 3 }],
    ['src/a.ts:3-0', { path: 'src/a.ts', line: 3 }],
    ['src/a.ts#L0', { path: 'src/a.ts' }],
    ['src/a.ts:7-7', { path: 'src/a.ts', line: 7 }],
    ['src/a.ts:64-42', { path: 'src/a.ts', line: 42, endLine: 64 }],
    ['src/a.ts#heading', { path: 'src/a.ts' }],
  ]
  for (const [href, want] of table) assert.deepEqual(link(href), want, href)
})

test('an inline code span with a path is a file ref', () => {
  const table: [string, FileRef][] = [
    ['src/a.ts', { path: 'src/a.ts' }],
    ['./src/a.ts', { path: './src/a.ts' }],
    ['../a.ts', { path: '../a.ts' }],
    ['~/notes/todo.md', { path: '~/notes/todo.md' }],
    ['/tmp/page.html', { path: '/tmp/page.html' }],
    ['/Users/me/My Docs/plan.md', { path: '/Users/me/My Docs/plan.md' }],
    ['/Users/me/My Docs/plan.md:12', { path: '/Users/me/My Docs/plan.md', line: 12 }],
    ['main.go:42', { path: 'main.go', line: 42 }],
    ['main.go:42:7', { path: 'main.go', line: 42, col: 7 }],
    ['src/a.ts:10-20', { path: 'src/a.ts', line: 10, endLine: 20 }],
    ['src/a.ts#L10-L20', { path: 'src/a.ts', line: 10, endLine: 20 }],
    ['src/a.ts#L10C3', { path: 'src/a.ts', line: 10, col: 3 }],
    ['Makefile:3', { path: 'Makefile', line: 3 }],
    ['build/Dockerfile', { path: 'build/Dockerfile' }],
    ['.github/workflows/ci.yml', { path: '.github/workflows/ci.yml' }],
    ['pkg/.env', { path: 'pkg/.env' }],
    ['app/[id]/page.tsx', { path: 'app/[id]/page.tsx' }],
    ['src/a.ts:0', { path: 'src/a.ts' }],
    ['src/a.test.ts', { path: 'src/a.test.ts' }],
  ]
  for (const [text, want] of table) assert.deepEqual(code(text), want, text)
})

test('an inline code span that is not a file path is no file ref', () => {
  const table = [
    '',
    'main.go',
    'README.md',
    'Makefile',
    'src/lib',
    'src/lib/',
    '/tmp/dir/',
    'src/my file.ts',
    'a.ts b.ts',
    'go test ./...',
    'cat src/a.ts',
    '/usr/bin/env\tnode',
    'src/<name>.ts',
    'src/a>b.ts',
    'src/`a`.ts',
    'src/**/*.ts',
    'src/?.ts',
    'src/{a,b}.ts',
    'example.com/readme.md',
    'example.com:8080',
    'localhost:3000',
    'localhost/a.html',
    'https://example.com/a.png',
    'git://host/a.git',
    '1.2.3',
    'v1.2.3',
    'v1.2.3/notes.md',
    '127.0.0.1:3000',
    'node_modules/1.2.3',
    'a/b',
    'and/or',
    'TCP/IP',
    '1/2',
    '.../a.ts',
    'src/…/a.ts',
    'src/a.ts#intro',
    'src/a.',
    ':42',
    '/',
    'a.ts:abc',
  ]
  for (const text of table) assert.equal(code(text), null, text)
})

test('a placed code span with a first step that is no host keeps its place', () => {
  assert.deepEqual(code('index.html:3'), { path: 'index.html', line: 3 })
  assert.deepEqual(code('server.py:10-12'), { path: 'server.py', line: 10, endLine: 12 })
})

test('refText writes a path with its place', () => {
  assert.equal(refText('/a/b.ts', {}), '/a/b.ts')
  assert.equal(refText('/a/b.ts', { line: 4 }), '/a/b.ts:4')
  assert.equal(refText('/a/b.ts', { line: 4, col: 2 }), '/a/b.ts:4:2')
  assert.equal(refText('/a/b.ts', { line: 4, endLine: 9 }), '/a/b.ts:4-9')
})

test('placeLabel is the place after a chip name', () => {
  assert.equal(placeLabel({}), '')
  assert.equal(placeLabel({ line: 42 }), ':42')
  assert.equal(placeLabel({ line: 42, endLine: 64 }), ':42–64')
})

test('chipNames adds the shortest folder that tells paths of one name apart', () => {
  const names = chipNames(['/w/src/index.ts', '/w/docs/index.ts', '/w/docs/plan.md', '/w/a/x/mod.go', '/w/b/x/mod.go', '/w/src/index.ts'])
  assert.equal(names.get('/w/src/index.ts'), 'src/index.ts')
  assert.equal(names.get('/w/docs/index.ts'), 'docs/index.ts')
  assert.equal(names.get('/w/docs/plan.md'), 'plan.md')
  assert.equal(names.get('/w/a/x/mod.go'), 'a/x/mod.go')
  assert.equal(names.get('/w/b/x/mod.go'), 'b/x/mod.go')
})

test('chipNames gives one name to a path named twice', () => {
  assert.equal(chipNames(['/w/a.ts', '/w/a.ts']).get('/w/a.ts'), 'a.ts')
})
