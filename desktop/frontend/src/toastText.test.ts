import assert from 'node:assert/strict'
import { test } from 'node:test'
import { cleanupToast, threadName, warningToast } from './toastText.ts'
import type { Cleanup } from './types.ts'

const open = () => {}
const entry = (over: Partial<Cleanup>): Cleanup => ({ at: 1, threadTitle: '#412 locale sweep', issue: 412, pr: 441, url: 'https://github.com/acme/app/pull/441', merged: true, status: 'done', removed: ['worktree /x/issue-412', 'thread put to sleep'], reason: '', ...over })

test('an issue thread is named by its issue and title, a thread without one by its quoted title', () => {
  assert.equal(threadName('#412 locale sweep', 412).name, '#412 locale sweep')
  assert.equal(threadName('locale sweep', 412).name, '#412 locale sweep')
  assert.equal(threadName('test', 0).name, 'Thread "test"')
  assert.equal(threadName('test', 0).full, undefined)
})

test('a long title is cut in the name and whole in full', () => {
  const title = 'Split the product page into sections and move the gallery'
  assert.equal(threadName(title, 0).name, 'Thread "Split the product page into sections an…"')
  assert.equal(threadName(title, 0).full, `Thread "${title}"`)
  assert.equal(threadName(`#9 ${title}`, 9).name, '#9 Split the product page into sections…')
})

test('a clean-up after a merged PR is a success that names the PR and what went', () => {
  const toast = cleanupToast(entry({}), open)
  assert.equal(toast.tone, 'success')
  assert.equal(toast.text, 'Cleaned up #412 locale sweep')
  assert.equal(toast.detail, 'PR #441 merged · Worktree and branch removed')
})

test('a clean-up after a closed PR says closed', () => {
  assert.equal(cleanupToast(entry({ merged: false }), open).detail, 'PR #441 closed · Worktree and branch removed')
})

test('a thread without an issue or a PR is quoted, and the detail says only what went', () => {
  const toast = cleanupToast(entry({ threadTitle: 'test', issue: 0, pr: 0, merged: false, removed: ['thread put to sleep', 'temp folder'], url: '' }), open)
  assert.equal(toast.text, 'Cleaned up Thread "test"')
  assert.equal(toast.detail, 'Thread put to sleep')
})

test('a clean-up with a PR has an Open action that opens it, and one without has none', () => {
  const opened: string[] = []
  const action = cleanupToast(entry({}), (url) => opened.push(url)).action
  assert.equal(action?.label, 'Open')
  action?.run()
  assert.deepEqual(opened, ['https://github.com/acme/app/pull/441'])
  assert.equal(cleanupToast(entry({ pr: 0, url: '' }), open).action, undefined)
})

test('a warning names its source in the text and keeps the message in the detail', () => {
  assert.deepEqual(warningToast({ source: 'github', message: 'gh could not name the repo' }), { tone: 'warning', text: 'Problem with GitHub', detail: 'gh could not name the repo', sticky: true })
  assert.equal(warningToast({ source: 'tmux', message: 'x' }).tone, 'error')
})
