import assert from 'node:assert/strict'
import { test } from 'node:test'
import { activeStrip, currentStrip, hitStrip, inView, isNotched, minimapStrips, persistentGutter, plainText, preview, railHeight, stripScale, stripTop, wheelPixels } from './chatMinimap.ts'
import type { TimelineItem } from './chatTimeline.ts'

const user = (key: string, text = key, files: string[] = []): TimelineItem => ({ kind: 'user', key, at: 0, id: key, uuid: key, text, files, forkAt: '' })
const reply = (key: string, text: string): TimelineItem => ({ kind: 'reply', key, at: 0, id: key, uuid: '', text, streaming: false })
const failure = (key: string, text: string): TimelineItem => ({ kind: 'notice', key, at: 0, text, error: true })
const note = (key: string): TimelineItem => ({ kind: 'notice', key, at: 0, text: key, error: false })
const work = (key: string): TimelineItem => ({ kind: 'work', key, header: true, startedAt: 0, endedAt: 0, calls: [], steps: [] })

test('a strip for each message and each error, in order, with its preview', () => {
  const items = [user('a', 'Fix the **cart**'), work('w'), reply('r1', 'Looking.'), reply('r2', 'Done: see [the PR](https://x/1).'), failure('e', 'API Error: 529\nOverloaded'), note('n'), user('b', '', ['img.png']), reply('r3', 'A picture.')]
  const strips = minimapStrips(items)
  assert.deepEqual(strips.map(({ key, index, kind }) => ({ key, index, kind })), [
    { key: 'a', index: 0, kind: 'prompt' },
    { key: 'e', index: 4, kind: 'error' },
    { key: 'b', index: 6, kind: 'prompt' },
  ])
  assert.deepEqual(strips.map(preview), [
    { title: 'Fix the cart', detail: 'Done: see the PR.' },
    { title: 'API Error: 529', detail: '' },
    { title: 'User message', detail: 'A picture.' },
  ])
})

test('markdown reads as plain text', () => {
  assert.equal(plainText('# Title\n\n- one\n- two\n\n> quoted `code`'), 'Title one two quoted code')
  assert.equal(plainText('See ![a chart](c.png) and <https://x.dev>'), 'See a chart and https://x.dev')
  assert.equal(plainText('```ts\nconst a = 1\n```'), 'const a = 1')
  assert.equal(plainText('a'.repeat(2000)).length, 1500, 'only the start of a long text is read')
})

test('strips sit 8 px apart until the rail is squeezed', () => {
  assert.equal(railHeight(2, 800), 8)
  assert.equal(railHeight(10, 800), 72)
  assert.equal(railHeight(200, 800), 564, 'the rail leaves 236 px of the pane free')
  assert.equal(railHeight(200, 200), 64, 'never shorter than 64 px')
  assert.deepEqual([0, 1, 2].map((i) => stripTop(i, 3, 16)), [0, 8, 16])
  const squeezed = railHeight(101, 536)
  assert.equal(squeezed, 300)
  assert.equal(stripTop(50, 101, squeezed), 150, 'by place in the order, not by where the item is')
})

test('the strip nearest the pointer is active', () => {
  assert.equal(activeStrip(0, 80, 11), 0)
  assert.equal(activeStrip(43, 80, 11), 5)
  assert.equal(activeStrip(45, 80, 11), 6)
  assert.equal(activeStrip(-20, 80, 11), 0, 'above the rail is the first')
  assert.equal(activeStrip(500, 80, 11), 10, 'below the rail is the last')
})

test('the active strip and its neighbours grow', () => {
  assert.deepEqual([0, 1, 2, 3, 9].map(stripScale), [1, 0.667, 0.417, 0.333, 0.333])
  assert.equal(stripScale(null), 0.333)
})

test('the hit strip fits the gutter', () => {
  assert.deepEqual(hitStrip(200), { left: 32, width: 40 })
  assert.deepEqual(hitStrip(50), { left: 32, width: 18 })
  assert.deepEqual(hitStrip(46), { left: 32, width: 14 })
  assert.deepEqual(hitStrip(45), { left: 0, width: 8 })
  assert.deepEqual(hitStrip(5), { left: 0, width: 5 })
  assert.equal(persistentGutter(48), true)
  assert.equal(persistentGutter(47), false)
})

// Rows: a message 0–100, a reply 100–900, an error 900–950, a message 950–1050, its reply 1050–1500.
const items = [user('a'), reply('r1', 'x'), failure('e', 'x'), user('b'), reply('r2', 'x')]
const rows = [
  { start: 0, end: 100 },
  { start: 100, end: 900 },
  { start: 900, end: 950 },
  { start: 950, end: 1050 },
  { start: 1050, end: 1500 },
]
const strips = minimapStrips(items)

test('a strip is in view while any row of its turn is', () => {
  assert.deepEqual(inView(strips, rows, 0, 400), [true, false, false])
  assert.deepEqual(inView(strips, rows, 500, 800), [true, false, false], 'a long reply keeps its message lit')
  assert.deepEqual(inView(strips, rows, 800, 1000), [true, true, true], 'several can be lit at once')
  assert.deepEqual(inView(strips, rows, 1200, 1500), [false, false, true])
  assert.deepEqual(inView(strips, rows.slice(0, 2), 0, 400), [true, false, false], 'a row not yet measured is not in view')
})

test('the current strip is the first whose row shows, else the one being read', () => {
  assert.equal(currentStrip(strips, rows, 0, 400), 0)
  assert.equal(currentStrip(strips, rows, 500, 800), 0, 'the reply of the first message is being read')
  assert.equal(currentStrip(strips, rows, 800, 1100), 1)
  assert.equal(currentStrip(strips, rows, 926, 1300), 2, 'a row in the top 25 px does not count')
  assert.equal(currentStrip(strips, rows, 1100, 1500), 2)
})

test('a notched wheel glides; a trackpad does not', () => {
  assert.equal(isNotched(0, -120), true)
  assert.equal(isNotched(0, -240), true)
  assert.equal(isNotched(1, 0), true, 'lines')
  assert.equal(isNotched(0, -9), false)
  assert.equal(isNotched(0, 0), false)
  assert.equal(wheelPixels(3, 1, 600), 48)
  assert.equal(wheelPixels(1, 2, 600), 600)
  assert.equal(wheelPixels(-37.5, 0, 600), -37.5)
})
