import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ANCHOR_SLACK, END_SLACK, followScroll, isAtBottom, keepScroll, keptScroll, stillFollows } from './chatScroll.ts'

test('at bottom within 48 px', () => {
  assert.equal(isAtBottom(1000, 500, 1500), true)
  assert.equal(isAtBottom(952, 500, 1500), true)
  assert.equal(isAtBottom(951, 500, 1500), false)
  assert.equal(isAtBottom(0, 500, 400), true, 'a chat shorter than its view is at the bottom')
  assert.equal(isAtBottom(990, 500, 1500, 0), false)
})

test('a following chat scrolls to its end when the composer grows', () => {
  assert.equal(followScroll({ following: true, grew: true, insetGrew: true, fromEnd: 0 }), 'end')
  assert.equal(followScroll({ following: false, grew: true, insetGrew: true, fromEnd: 0 }), 'stay', 'a reader scrolled up stays')
})

test('a following chat the virtualizer does not anchor scrolls to its end as it grows', () => {
  assert.equal(followScroll({ following: true, grew: true, insetGrew: false, fromEnd: ANCHOR_SLACK }), 'stay', 'anchored in the frame it grew')
  assert.equal(followScroll({ following: true, grew: true, insetGrew: false, fromEnd: ANCHOR_SLACK + 1 }), 'end', 'inside the follow slack, past the anchor')
  assert.equal(followScroll({ following: true, grew: false, insetGrew: false, fromEnd: END_SLACK }), 'stay', 'nothing grew')
  assert.equal(followScroll({ following: false, grew: true, insetGrew: false, fromEnd: 500 }), 'stay', 'a reader scrolled up stays')
})

test('only scrolling up stops the follow', () => {
  const view = { client: 500, height: 1500 }
  assert.equal(stillFollows({ ...view, following: false, top: 1000 - END_SLACK, lastTop: 0 }), true, 'back at the end it follows again')
  assert.equal(stillFollows({ ...view, following: true, top: 600, lastTop: 600 }), true, 'a row that grew moved the end away')
  assert.equal(stillFollows({ ...view, following: true, top: 599, lastTop: 600 }), false, 'scrolled up')
  assert.equal(stillFollows({ ...view, following: false, top: 700, lastTop: 600 }), false, 'scrolled down, short of the end')
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
