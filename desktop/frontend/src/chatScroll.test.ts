import assert from 'node:assert/strict'
import { test } from 'node:test'
import { followScroll, isAtBottom, keepScroll, keptScroll } from './chatScroll.ts'

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
  keepScroll('p/a', { top: 640, measured })
  assert.deepEqual(keptScroll('p/a'), { top: 640, measured })

  keepScroll('p/a', null)
  assert.equal(keptScroll('p/a'), null, 'a thread left at the bottom opens at the bottom again')

  for (let i = 0; i <= 20; i++) keepScroll(`p/${i}`, { top: i, measured: [] })
  assert.equal(keptScroll('p/0'), null, 'only the latest 20 threads are kept')
  assert.equal(keptScroll('p/20')?.top, 20)
})
