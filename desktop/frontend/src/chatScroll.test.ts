import assert from 'node:assert/strict'
import { test } from 'node:test'
import { followScroll, hasUnseen, isAtBottom, keepScroll, keptScroll, minimapMarks, thumb, trackToScroll } from './chatScroll.ts'
import type { TimelineItem } from './chatTimeline.ts'

test('at bottom within 48 px', () => {
  assert.equal(isAtBottom(1000, 500, 1500), true)
  assert.equal(isAtBottom(952, 500, 1500), true)
  assert.equal(isAtBottom(951, 500, 1500), false)
  assert.equal(isAtBottom(0, 500, 400), true, 'a chat shorter than its view is at the bottom')
  assert.equal(isAtBottom(990, 500, 1500, 0), false)
})

test('follows only when at the bottom', () => {
  assert.equal(followScroll({ atBottom: true, grew: true }), 'end')
  assert.equal(followScroll({ atBottom: false, grew: true }), 'stay')
  assert.equal(followScroll({ atBottom: true, grew: false }), 'stay')
  assert.equal(followScroll({ atBottom: false, grew: false }), 'stay')
})

test('restores a kept position', () => {
  assert.equal(keptScroll('p/none'), null, 'a thread never scrolled opens at the bottom')

  const measured = [{ index: 0, key: 'user-u-1', start: 24, size: 80, end: 104, lane: 0 }]
  keepScroll('p/a', { top: 640, measured, count: 12 })
  assert.deepEqual(keptScroll('p/a'), { top: 640, measured, count: 12 })

  keepScroll('p/a', null)
  assert.equal(keptScroll('p/a'), null, 'a thread left at the bottom opens at the bottom again')

  for (let i = 0; i <= 20; i++) keepScroll(`p/${i}`, { top: i, measured: [], count: 0 })
  assert.equal(keptScroll('p/0'), null, 'only the latest 20 threads are kept')
  assert.equal(keptScroll('p/20')?.top, 20)
})

test('items past the count last seen are unseen', () => {
  const kept = { top: 640, measured: [], count: 12 }
  assert.equal(hasUnseen(kept, 12), false)
  assert.equal(hasUnseen(kept, 14), true)
  assert.equal(hasUnseen(null, 14), false, 'a thread that opens at the bottom shows its newest items')
})

const user = (key: string): TimelineItem => ({ kind: 'user', key, at: 0, id: key, text: key, files: [] })
const approval = (key: string): TimelineItem => ({ kind: 'approval', key, at: 0, request: key, tool: 'Bash', input: {}, expired: false })
const failure = (key: string): TimelineItem => ({ kind: 'notice', key, at: 0, text: key, error: true })
const note = (key: string): TimelineItem => ({ kind: 'notice', key, at: 0, text: key, error: false })

test('marks scale to the bar', () => {
  const marks = minimapMarks([user('a'), note('n'), approval('b'), failure('c')], [0, 250, 500, 1000], 2000, 400)
  assert.deepEqual(marks, [
    { top: 0, kind: 'user', key: 'a' },
    { top: 100, kind: 'approval', key: 'b' },
    { top: 200, kind: 'error', key: 'c' },
  ])
})

test('close marks merge by priority', () => {
  const items = [user('a'), failure('b'), approval('c'), user('d')]
  assert.deepEqual(minimapMarks(items, [0, 10, 20, 50], 1000, 100), [{ top: 2, kind: 'approval', key: 'c' }, { top: 5, kind: 'user', key: 'd' }])
  assert.deepEqual(minimapMarks([user('a'), user('b'), user('c'), user('d')], [0, 20, 40, 60], 1000, 100).map((m) => m.key), ['a', 'c'], 'a run of close marks does not drift')
  assert.deepEqual(minimapMarks([approval('a'), user('b')], [0, 10], 1000, 100), [{ top: 0, kind: 'approval', key: 'a' }])
})

test('thumb minimum size', () => {
  assert.deepEqual(thumb(0, 500, 1000, 400), { top: 0, size: 200 })
  assert.deepEqual(thumb(500, 500, 1000, 400), { top: 200, size: 200 })
  assert.deepEqual(thumb(99500, 500, 100000, 400), { top: 376, size: 24 })
  assert.deepEqual(thumb(0, 500, 300, 400), { top: 0, size: 400 })
})

test('track click centres', () => {
  assert.equal(trackToScroll(200, 400, 2000, 500), 750)
  assert.equal(trackToScroll(0, 400, 2000, 500), 0)
  assert.equal(trackToScroll(400, 400, 2000, 500), 1500)
})
