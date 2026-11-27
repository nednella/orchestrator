import assert from 'node:assert/strict'
import { test } from 'node:test'
import { imageTarget, linkTarget, onPicked, pickedFile, pickFile, pickFolder, pickOf, planPath } from './filesView.ts'

test('a relative markdown link is a file', () => {
  assert.equal(planPath('docs/plan.md'), 'docs/plan.md')
  assert.equal(planPath('./PLAN.markdown'), './PLAN.markdown')
  assert.equal(planPath('../notes.MD'), '../notes.MD')
  assert.equal(planPath('docs/plan.md#steps'), 'docs/plan.md', 'without its fragment')
  assert.equal(planPath('docs/my%20plan.md'), 'docs/my plan.md', 'decoded')
  assert.equal(planPath('/Users/sam/trees/issue-7/plan.md'), '/Users/sam/trees/issue-7/plan.md')
  assert.equal(planPath('docs/main.go'), null, 'not markdown')
  assert.equal(planPath('#steps'), null)
  assert.equal(planPath(''), null)
})

test('web links are not', () => {
  for (const href of ['https://example.com/README.md', 'http://example.com/a.md', 'mailto:sam@example.com', '//example.com/a.md', 'javascript:alert(1)//.md', 'vscode://file/a.md', 'data:text/html,a.md']) {
    assert.equal(planPath(href), null, href)
  }
})

test('a file URL is decoded', () => {
  assert.equal(planPath('file:///Users/sam/trees/my%20tree/plan.md'), '/Users/sam/trees/my tree/plan.md')
  assert.equal(planPath('file://localhost/tmp/plan.md'), '/tmp/plan.md')
  assert.equal(planPath('file://server/share/plan.md'), null, 'another machine')
  assert.equal(planPath('file:///tmp/plan.txt'), null)
})

test('a link in a file points from its folder', () => {
  assert.equal(linkTarget('docs/config.md', 'testing.md'), 'docs/testing.md')
  assert.equal(linkTarget('docs/config.md', '../AGENTS.md'), 'AGENTS.md')
  assert.equal(linkTarget('docs/config.md', './a/../b.md'), 'docs/b.md')
  assert.equal(linkTarget('README.md', '../../x.md'), '../../x.md', 'past the top stays')
  assert.equal(linkTarget('', 'docs/plan.md'), 'docs/plan.md', 'from the chat')
  assert.equal(linkTarget('docs/config.md', '/tmp/plan.md'), '/tmp/plan.md')
})

test('an image of a markdown file is a path in its folder', () => {
  assert.equal(imageTarget('docs/guide.md', 'logo.svg'), 'docs/logo.svg')
  assert.equal(imageTarget('docs/guide.md', '../assets/my%20logo.png?raw=1#x'), 'assets/my logo.png')
  assert.equal(imageTarget('README.md', './assets/logo.svg'), 'assets/logo.svg')
  assert.equal(imageTarget('README.md', '../../out.png'), '../../out.png', 'past the top stays, for the app to refuse')
})

test('a web, data or absolute image is not one', () => {
  for (const src of ['https://example.com/a.png', 'http://example.com/a.png', '//example.com/a.png', 'data:image/png;base64,AAAA', 'file:///tmp/a.png', '/tmp/a.png', '', '#x']) {
    assert.equal(imageTarget('README.md', src), null, src)
  }
})

test('a picked file opens in its own view, and null closes it there only', () => {
  let heard = 0
  const stop = onPicked(() => heard++)
  pickFile('thread-a', 'docs/plan.md')
  pickFile('thread-b', 'README.md')
  assert.equal(pickedFile('thread-a'), 'docs/plan.md')
  pickFile('thread-a', null)
  assert.equal(pickedFile('thread-a'), null)
  assert.equal(pickedFile('thread-b'), 'README.md')
  assert.equal(heard, 3, 'each pick and the close are heard')
  stop()
  pickFile('thread-b', null)
})

test('a pick keeps the place in the file and a folder pick closes the file', () => {
  pickFile('thread-p', 'src/a.ts', { line: 4, endLine: 9 })
  assert.deepEqual(pickOf('thread-p'), { path: 'src/a.ts', folder: null, place: { line: 4, endLine: 9 } })
  pickFile('thread-p', 'src/a.ts')
  assert.equal(pickOf('thread-p')?.place, null)
  const before = pickOf('thread-p')
  pickFile('thread-p', 'src/a.ts')
  assert.equal(pickOf('thread-p') !== before, true, 'the same file picked again is a new pick')
  let heard = 0
  const off = onPicked(() => heard++)
  pickFolder('thread-p', 'src')
  assert.deepEqual(pickOf('thread-p'), { path: null, folder: 'src', place: null })
  assert.equal(pickedFile('thread-p'), null)
  assert.equal(heard, 1)
  off()
  pickFile('thread-p', null)
  assert.equal(pickOf('thread-p'), null)
})
