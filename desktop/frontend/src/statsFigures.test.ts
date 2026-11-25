import assert from 'node:assert/strict'
import { test } from 'node:test'
import { aDay, change, changeLabel, costAxis, counted, kindCounts, periodBefore, workAxis, workTime, workTitle } from './statsFigures.ts'

test('the change on the period before is in whole percent, up or down', () => {
  assert.deepEqual(change(112, 100), { direction: 'up', text: '12%' })
  assert.deepEqual(change(96, 100), { direction: 'down', text: '4%' })
  assert.deepEqual(change(100.4, 100), { direction: 'flat', text: '0%' })
  assert.deepEqual(change(5000, 100), { direction: 'up', text: '4,900%' })
})

test('there is no change to show when the period before had nothing', () => {
  assert.equal(change(12, 0), null)
  assert.equal(change(0, 0), null)
})

test('the change reads with the period it compares with', () => {
  assert.equal(periodBefore(30), 'the 30 days before')
  assert.equal(periodBefore(1), 'yesterday')
  assert.equal(periodBefore(0), '')
  assert.equal(changeLabel({ direction: 'up', text: '12%' }, periodBefore(30)), 'Up 12% on the 30 days before')
  assert.equal(changeLabel({ direction: 'down', text: '4%' }, periodBefore(1)), 'Down 4% on yesterday')
  assert.equal(changeLabel({ direction: 'flat', text: '0%' }, periodBefore(7)), 'No change on the 7 days before')
})

test('worked time is hours and minutes, never days', () => {
  assert.equal(workTime((61 * 60 + 20) * 60_000), '61h 20m')
  assert.equal(workTime(3 * 3_600_000), '3h')
  assert.equal(workTime(42 * 60_000), '42m')
  assert.equal(workTime(9_000), '9s')
  assert.equal(workTime(0), '0m')
  assert.equal(workTime(148.4 * 3_600_000), '148h')
  assert.equal(workTime(1_234 * 3_600_000), '1,234h')
  assert.equal(workTime(72_000, true), '1m 12s')
  assert.equal(workTime(120_000, true), '2m')
  assert.equal(workTime((9 * 60 + 40) * 60_000 + 5_000, true), '9h 40m')
})

test('a day is one decimal under ten, whole above', () => {
  assert.equal(aDay(1209, 30), '40')
  assert.equal(aDay(214, 30), '7.1')
  assert.equal(aDay(60, 30), '2')
  assert.equal(aDay(0, 30), '0')
  assert.equal(aDay(5, 0), '5')
})

test('axis values fit the top of the chart', () => {
  assert.equal(workAxis(5 * 3_600_000, 5 * 3_600_000), '5h')
  assert.equal(workAxis(45 * 60_000, 90 * 60_000), '45m')
  assert.equal(costAxis(15.76, 15.76), '$16')
  assert.equal(costAxis(8, 15.76), '$8')
  assert.equal(costAxis(0.4, 0.8), '$0.40')
})

test('a work row puts its issue number apart from its title', () => {
  assert.deepEqual(workTitle({ issue: 549, title: '#549 Browser tab per thread' }), { ref: '#549', title: 'Browser tab per thread' })
  assert.deepEqual(workTitle({ issue: 549, title: 'Renamed by hand' }), { ref: '#549', title: 'Renamed by hand' })
  assert.deepEqual(workTitle({ issue: 0, title: '#12 looks like an issue' }), { ref: '', title: '#12 looks like an issue' })
})

test('waits add up by kind, most first', () => {
  const causes = [
    { kind: 'idle' as const, count: 72 },
    { kind: 'question' as const, count: 38 },
    { kind: 'permission' as const, count: 31 },
    { kind: 'question' as const, count: 23 },
    { kind: 'permission' as const, count: 50 },
  ]
  assert.deepEqual(kindCounts(causes), [
    { kind: 'permission', count: 81 },
    { kind: 'idle', count: 72 },
    { kind: 'question', count: 61 },
  ])
  assert.deepEqual(kindCounts([]), [])
})

test('a count takes the noun for one or many', () => {
  assert.equal(counted(1, 'issue', 'issues'), '1 issue')
  assert.equal(counted(0, 'chat', 'chats'), '0 chats')
  assert.equal(counted(1209, 'prompt', 'prompts'), '1,209 prompts')
})
