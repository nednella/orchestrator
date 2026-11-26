import assert from 'node:assert/strict'
import { test } from 'node:test'
import { fileLabel, LATEST_PLAN, linkTarget, pickerOrder, planPath } from './filesView.ts'

test('a relative markdown link is a file', () => {
  assert.equal(planPath('docs/plan.md'), 'docs/plan.md')
  assert.equal(planPath('./PLAN.markdown'), './PLAN.markdown')
  assert.equal(planPath('../notes.MD'), '../notes.MD')
  assert.equal(planPath('docs/plan.md#steps'), 'docs/plan.md', 'without its fragment')
  assert.equal(planPath('docs/my%20plan.md'), 'docs/my plan.md', 'decoded')
  assert.equal(planPath('/Users/ned/trees/issue-7/plan.md'), '/Users/ned/trees/issue-7/plan.md')
  assert.equal(planPath('docs/main.go'), null, 'not markdown')
  assert.equal(planPath('#steps'), null)
  assert.equal(planPath(''), null)
})

test('web links are not', () => {
  for (const href of ['https://example.com/README.md', 'http://example.com/a.md', 'mailto:ned@example.com', '//example.com/a.md', 'javascript:alert(1)//.md', 'vscode://file/a.md', 'data:text/html,a.md']) {
    assert.equal(planPath(href), null, href)
  }
})

test('a file URL is decoded', () => {
  assert.equal(planPath('file:///Users/ned/trees/my%20tree/plan.md'), '/Users/ned/trees/my tree/plan.md')
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

test('AGENTS.md and README.md come first', () => {
  const files = ['src/z.md', 'docs/testing.md', 'README.md', 'CHANGELOG.md', 'docs/config.md', 'AGENTS.md', 'desktop/AGENTS.md']
  assert.deepEqual(pickerOrder(files), ['AGENTS.md', 'README.md', 'docs/config.md', 'docs/testing.md', 'CHANGELOG.md', 'desktop/AGENTS.md', 'src/z.md'])
  assert.deepEqual(files[0], 'src/z.md', 'the list given is left as it was')
})

test('the picker names a file by its folders and its name', () => {
  assert.deepEqual(fileLabel('docs/guides/setup.md'), { folders: 'docs / guides', name: 'setup.md' })
  assert.deepEqual(fileLabel('AGENTS.md'), { folders: '', name: 'AGENTS.md' })
  assert.deepEqual(fileLabel('/tmp/plan.md'), { folders: 'tmp', name: 'plan.md' })
  assert.deepEqual(fileLabel(LATEST_PLAN), { folders: '', name: 'Latest plan' })
})
