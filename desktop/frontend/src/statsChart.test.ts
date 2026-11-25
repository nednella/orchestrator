import assert from 'node:assert/strict'
import { test } from 'node:test'
import { busiest, chartBars, dayName, rangeLabel, tipAlign, weekly } from './statsChart.ts'
import type { DayFigures } from './types.ts'

// Days from 2026-10-27, a Tuesday, each with the prompts given.
function days(prompts: number[], first = new Date(2026, 9, 27)): DayFigures[] {
  return prompts.map((n, i) => {
    const d = new Date(first.getFullYear(), first.getMonth(), first.getDate() + i)
    const day = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    return { day, prompts: n, workMs: n * 60_000, cost: n / 10 }
  })
}

test('a bar a day, each named for its day, the last one today so far', () => {
  const bars = chartBars(days([3, 0, 5]), 'prompts')
  assert.deepEqual(
    bars.map((b) => [b.label, b.value]),
    [
      ['Tue 27 Oct', 3],
      ['Wed 28 Oct', 0],
      ['Thu 29 Oct (today, so far)', 5],
    ],
  )
  assert.deepEqual(
    chartBars(days([3, 0, 5]), 'worked').map((b) => b.value),
    [180_000, 0, 300_000],
  )
  assert.deepEqual(
    chartBars(days([3, 0, 5]), 'cost').map((b) => b.value),
    [0.3, 0, 0.5],
  )
})

test('a month of days is ticked on Mondays; a week, on every day', () => {
  const month = chartBars(days(Array(30).fill(1)), 'prompts')
  assert.deepEqual(
    month.filter((b) => b.tick).map((b) => b.tick),
    ['2 Nov', '9 Nov', '16 Nov', '23 Nov'],
  )
  const week = chartBars(days(Array(7).fill(1)), 'prompts')
  assert.equal(week.filter((b) => b.tick).length, 7)
})

test('a long range has a bar a week from Monday, ticked where a month starts', () => {
  const year = days(Array(365).fill(1), new Date(2025, 10, 26))
  assert.equal(weekly(year), true)
  assert.equal(weekly(days(Array(92).fill(1))), false)
  const bars = chartBars(year, 'prompts')
  assert.equal(bars[0].label, 'Week of 26 Nov')
  assert.equal(bars[0].value, 5, 'Wednesday to Sunday')
  assert.equal(bars[1].label, 'Week of 1 Dec')
  assert.equal(bars[1].value, 7)
  assert.equal(bars.reduce((a, b) => a + b.value, 0), 365)
  assert.equal(bars.at(-1)!.label, 'Week of 23 Nov (this week, so far)')
  const ticks = bars.filter((b) => b.tick).map((b) => b.tick)
  assert.equal(ticks[0], 'Nov')
  assert.equal(ticks.includes('2026'), true)
  assert.equal(ticks.length <= 14, true)
})

test('the busiest bar is the first with the most, none when all are empty', () => {
  assert.equal(busiest(chartBars(days([1, 4, 2, 4]), 'prompts'), false), 'most on Wed 28 Oct')
  assert.equal(busiest(chartBars(days([0, 0]), 'prompts'), false), '')
  assert.equal(busiest([], false), '')
  const year = chartBars(days(Array(365).fill(1), new Date(2025, 10, 26)), 'prompts')
  assert.equal(busiest(year, true), 'most in the week of 1 Dec')
})

test('a tip near either end stays inside the chart', () => {
  assert.equal(tipAlign(0, 30), 'start')
  assert.equal(tipAlign(15, 30), 'center')
  assert.equal(tipAlign(29, 30), 'end')
})

test('the range reads as its first and last day', () => {
  assert.equal(rangeLabel('2026-10-27', '2026-11-25'), '27 Oct – 25 Nov')
  assert.equal(rangeLabel('2025-11-26', '2026-11-25'), '26 Nov 2025 – 25 Nov 2026')
  assert.equal(rangeLabel('2026-11-25', '2026-11-25'), 'Wed 25 Nov')
  assert.equal(dayName('2026-11-11'), 'Wed 11 Nov')
})
