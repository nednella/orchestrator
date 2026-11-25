import assert from 'node:assert/strict'
import { test } from 'node:test'
import { messageTime, messageTimeTitle } from './messageTime.ts'

const at = (y: number, m: number, d: number, h: number, min: number) => new Date(y, m - 1, d, h, min).getTime()
const now = at(2026, 11, 25, 16, 0)

test('a message from today shows its time in the locale', () => {
  assert.equal(messageTime(at(2026, 11, 25, 14, 54), now, 'en-GB'), '14:54')
  assert.equal(messageTime(at(2026, 11, 25, 10, 5), now, 'en-GB'), '10:05')
  assert.equal(messageTime(at(2026, 11, 25, 14, 54), now, 'en-US'), '2:54 PM')
})

test('a message from yesterday says so', () => {
  assert.equal(messageTime(at(2026, 11, 24, 23, 59), now, 'en-GB'), 'yesterday at 23:59')
  assert.equal(messageTime(at(2026, 11, 30, 18, 0), at(2026, 12, 1, 1, 0), 'en-GB'), 'yesterday at 18:00')
})

test('an older message shows the day and month, and the year once it is another year', () => {
  assert.equal(messageTime(at(2026, 11, 20, 14, 54), now, 'en-GB'), '20/11, 14:54')
  assert.equal(messageTime(at(2025, 12, 31, 14, 54), at(2026, 1, 2, 9, 0), 'en-GB'), '31/12/2025, 14:54')
})

test('the long form gives the day with its ordinal, the month and the year', () => {
  assert.equal(messageTimeTitle(at(2026, 11, 25, 14, 54), 'en-GB'), '14:54, 25th November 2026')
  assert.equal(messageTimeTitle(at(2026, 11, 1, 14, 54), 'en-GB'), '14:54, 1st November 2026')
  assert.equal(messageTimeTitle(at(2026, 11, 22, 14, 54), 'en-GB'), '14:54, 22nd November 2026')
  assert.equal(messageTimeTitle(at(2026, 11, 13, 14, 54), 'en-GB'), '14:54, 13th November 2026')
})
