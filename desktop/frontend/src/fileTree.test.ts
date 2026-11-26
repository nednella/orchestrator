import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ancestorsOf, childrenOf, crumbsOf, formatBytes, isMarkdownPath, onlyChildFolders, parentOf, relativeTo, treePath, withFolders, withoutFrontMatter } from './fileTree.ts'
import type { FileEntry } from './types.ts'

const file = (path: string, ignored = false): FileEntry => ({ path, dir: false, ignored })
const dir = (path: string, ignored = false): FileEntry => ({ path, dir: true, ignored })

test('a folder is named with a slash at its end', () => {
  assert.equal(treePath(dir('src')), 'src/')
  assert.equal(treePath(file('src/a.ts')), 'src/a.ts')
})

test('the folders a path is in come outermost first', () => {
  assert.deepEqual(ancestorsOf('a/b/c.ts'), ['a/', 'a/b/'])
  assert.deepEqual(ancestorsOf('a/b/'), ['a/'])
  assert.deepEqual(ancestorsOf('c.ts'), [])
})

test('the crumbs of a path end at its folders and then at the file', () => {
  assert.deepEqual(crumbsOf('src/app/index.ts'), [
    { name: 'src', folder: 'src' },
    { name: 'app', folder: 'src/app' },
    { name: 'index.ts', folder: '' },
  ])
  assert.deepEqual(crumbsOf('README.md'), [{ name: 'README.md', folder: '' }])
})

test('a size is said in the unit it fits', () => {
  assert.equal(formatBytes(900), '900 B')
  assert.equal(formatBytes(1536), '1.5 KB')
  assert.equal(formatBytes(48213), '47 KB')
  assert.equal(formatBytes(3 * 1024 * 1024), '3.0 MB')
})

test('the folders a list leads through are added once', () => {
  const entries = [file('a/b/c.ts'), file('a/d.ts'), dir('node_modules', true), file('top.md')]
  const all = withFolders(entries)
  assert.deepEqual(
    all.filter((e) => e.dir).map((e) => e.path),
    ['node_modules', 'a', 'a/b'],
  )
  assert.equal(withFolders(all), all, 'a list that has them is left as it is')
})

test('a folder holds its entries with its folders first', () => {
  const all = withFolders([file('src/b.ts'), file('src/a/x.ts'), file('src/a.ts'), file('srcx/y.ts'), file('top.md')])
  assert.deepEqual(
    childrenOf(all, 'src').map((e) => e.path),
    ['src/a', 'src/a.ts', 'src/b.ts'],
  )
  assert.deepEqual(
    childrenOf(all, '').map((e) => e.path),
    ['src', 'srcx', 'top.md'],
  )
})

test('a path has the folder it is in', () => {
  assert.equal(parentOf('a/b/c.ts'), 'a/b')
  assert.equal(parentOf('c.ts'), '')
})

test('a folder that holds only a folder is known', () => {
  const entries = withFolders([file('a/b/c.ts'), file('d/e.ts'), file('d/f.ts'), dir('node_modules', true), dir('node_modules/left-pad', true)])
  assert.deepEqual([...onlyChildFolders(entries)].sort(), [
    ['a', 'a/b'],
    ['node_modules', 'node_modules/left-pad'],
  ])
})

test('the file view takes a path inside the folder as the tree names it', () => {
  assert.equal(relativeTo('/work/shop', '/work/shop/src/a.ts'), 'src/a.ts')
  assert.equal(relativeTo('/work/shop', 'src/a.ts'), 'src/a.ts')
  assert.equal(relativeTo('/work/shop', '/work/shopper/a.ts'), '/work/shopper/a.ts')
  assert.equal(relativeTo('', '/tmp/a.ts'), '/tmp/a.ts')
})

test('markdown is told by the name', () => {
  for (const name of ['a.md', 'B.MDX', 'c.markdown', 'd.mdown', 'e.mkd']) assert.equal(isMarkdownPath(name), true, name)
  for (const name of ['a.txt', 'md', 'a.md.bak']) assert.equal(isMarkdownPath(name), false, name)
})

test('front matter is left out of a markdown file', () => {
  assert.equal(withoutFrontMatter('---\ntitle: x\n---\n# Title\n'), '# Title\n')
  assert.equal(withoutFrontMatter('---\r\ntitle: x\r\n---\r\nbody'), 'body')
  assert.equal(withoutFrontMatter('# Title\n---\nnot front matter\n---\n'), '# Title\n---\nnot front matter\n---\n')
  assert.equal(withoutFrontMatter('---\nnever closed\n'), '---\nnever closed\n')
})
