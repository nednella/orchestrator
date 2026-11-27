import assert from 'node:assert/strict'
import { test } from 'node:test'
import { clamp, hexToHsv, hsvToHex, normalizeHex, nudge, pointToHueSaturation, wheelPoint } from './accentWheel.ts'

const near = (a: number, b: number) => assert.equal(Math.abs(a - b) < 1e-9, true, `${a} is not ${b}`)

test('the primary colors sit at their hues', () => {
  assert.deepEqual(hexToHsv('#ff0000'), { h: 0, s: 1, v: 1 })
  assert.equal(hexToHsv('#00ff00').h, 120)
  assert.equal(hexToHsv('#0000ff').h, 240)
  assert.deepEqual(hexToHsv('#ffffff'), { h: 0, s: 0, v: 1 })
  assert.deepEqual(hexToHsv('#000000'), { h: 0, s: 0, v: 0 })
})

test('a color survives the way to hsv and back', () => {
  for (const hex of ['#65a7fa', '#ac8ff8', '#be222a', '#21763c', '#777777', '#010203', '#fefefe', '#7c5ce0']) {
    assert.equal(hsvToHex(hexToHsv(hex)), hex)
  }
})

test('hsv turns into the hex of its colors', () => {
  assert.equal(hsvToHex({ h: 0, s: 1, v: 1 }), '#ff0000')
  assert.equal(hsvToHex({ h: 60, s: 1, v: 1 }), '#ffff00')
  assert.equal(hsvToHex({ h: 360, s: 1, v: 1 }), '#ff0000')
  assert.equal(hsvToHex({ h: 200, s: 0, v: 0.5 }), '#808080')
  assert.equal(hsvToHex({ h: 200, s: 1, v: 0 }), '#000000')
})

test('a typed color is a hex with or without its #', () => {
  assert.equal(normalizeHex('#AbCdEf'), '#abcdef')
  assert.equal(normalizeHex(' 65a7fa '), '#65a7fa')
  for (const bad of ['', '#', '#fff', '#12345', '#1234567', '#ggg000', 'blue', '##123456']) assert.equal(normalizeHex(bad), null)
})

test('the wheel puts hue clockwise from the top and saturation outward', () => {
  const top = wheelPoint({ h: 0, s: 1, v: 1 })
  near(top.x, 0)
  near(top.y, -1)
  const right = wheelPoint({ h: 90, s: 0.5, v: 1 })
  near(right.x, 0.5)
  near(right.y, 0)
  const bottom = wheelPoint({ h: 180, s: 1, v: 1 })
  near(bottom.y, 1)
  near(wheelPoint({ h: 123, s: 0, v: 1 }).x, 0)
})

test('a place on the wheel gives back the hue and saturation that put it there', () => {
  for (const [h, s] of [[0, 1], [45, 0.3], [90, 0.5], [181, 0.9], [270, 0.75], [359, 0.2]]) {
    const back = pointToHueSaturation(wheelPoint({ h, s, v: 1 }))
    near(back.h, h)
    near(back.s, s)
  }
})

test('a place beyond the edge gives the edge', () => {
  const out = pointToHueSaturation({ x: 3, y: 0 })
  near(out.h, 90)
  assert.equal(out.s, 1)
  assert.deepEqual(pointToHueSaturation({ x: 0, y: 0 }), { h: 0, s: 0 })
})

test('an arrow key moves the handle one step across the wheel and keeps the brightness', () => {
  const moved = nudge({ h: 90, s: 0.5, v: 0.4 }, 'ArrowRight', 0.1)
  near(moved.s, 0.6)
  near(moved.h, 90)
  assert.equal(moved.v, 0.4)
  const up = nudge({ h: 90, s: 0.5, v: 1 }, 'ArrowUp', 0.1)
  assert.equal(up.h < 90 && up.s > 0.4, true)
})

test('the handle stays inside the wheel and a key that is not an arrow changes nothing', () => {
  assert.equal(nudge({ h: 90, s: 1, v: 1 }, 'ArrowRight', 0.1).s, 1)
  const centre = nudge({ h: 0, s: 0, v: 1 }, 'ArrowLeft', 0.1)
  near(centre.h, 270)
  near(centre.s, 0.1)
  const color = { h: 10, s: 0.5, v: 0.5 }
  assert.equal(nudge(color, 'a', 0.1), color)
})

test('clamp keeps a number in range', () => {
  assert.equal(clamp(-1, 0, 1), 0)
  assert.equal(clamp(2, 0, 1), 1)
  assert.equal(clamp(0.4, 0, 1), 0.4)
})
