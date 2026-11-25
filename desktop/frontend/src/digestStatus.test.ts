import assert from 'node:assert/strict'
import { test } from 'node:test'
import { formatWorked } from './chatTools.ts'
import { agoWords, runState, runStatus, runStatusEnd } from './digestStatus.ts'

const minute = 60_000
const day = 24 * 60 * minute
const now = new Date(2026, 10, 25, 14, 3, 12).getTime()
const never = { running: false, startedAt: 0, lastRunAt: 0, nextRunAt: 0, error: '', stoppedAt: 0 }

test('the run state comes from running, then the error, then whether it ever ran', () => {
  assert.equal(runState(never), 'never')
  assert.equal(runState({ ...never, lastRunAt: now }), 'checked')
  assert.equal(runState({ ...never, lastRunAt: now, error: 'claude exited' }), 'failed')
  assert.equal(runState({ ...never, error: 'claude exited', running: true }), 'running')
  assert.equal(runState({ ...never, startedAt: now }), 'failed')
  assert.equal(runState({ ...never, startedAt: now, lastRunAt: now - day }), 'failed')
  assert.equal(runState({ ...never, startedAt: now - day, lastRunAt: now }), 'checked')
})

test('ago reads in words', () => {
  assert.equal(agoWords(now - 20_000, now), 'just now')
  assert.equal(agoWords(now - minute, now), '1 minute ago')
  assert.equal(agoWords(now - 5 * 60 * minute, now), '5 hours ago')
  assert.equal(agoWords(now - 2 * day, now), '2 days ago')
  assert.equal(agoWords(now + minute, now), 'just now')
})

test('a finished run says when it ran and how long it took', () => {
  const lastRunAt = now - 2 * day
  assert.equal(runStatus({ ...never, startedAt: lastRunAt - 124_000, lastRunAt }, now, formatWorked), 'Checked 2 days ago in 2m 4s')
})

test('a run that ran before start times were kept says only when it ran', () => {
  assert.equal(runStatus({ ...never, lastRunAt: now - 2 * day }, now, formatWorked), 'Checked 2 days ago')
})

test('a run that failed before a restart, which forgot why, says the last run failed', () => {
  assert.equal(runStatus({ ...never, startedAt: now - day }, now, formatWorked), 'Last run failed')
  assert.equal(runStatus({ ...never, startedAt: now - day, lastRunAt: now - 2 * day }, now, formatWorked), 'Last run failed')
})

test('a running run says how long it has been checking and when it started', () => {
  const d = { ...never, running: true, startedAt: now - 72_000 }
  assert.equal(runStatus(d, now, formatWorked), 'Checking for 1m 12s')
  assert.equal(runStatusEnd(d, now), 'Started 14:02')
})

test('a failed run says how long it ran and when it stopped', () => {
  const stoppedAt = now - 2 * day
  assert.equal(runStatus({ ...never, error: 'claude exited', startedAt: stoppedAt - 124_000, stoppedAt }, now, formatWorked), 'Stopped after 2m 4s, 2 days ago')
})

test('a project that never ran says so', () => {
  assert.equal(runStatus(never, now, formatWorked), 'Never run')
})

test('the end of the row names the next run, or says runs are off', () => {
  assert.equal(runStatusEnd({ ...never, nextRunAt: new Date(2026, 10, 30, 9, 0).getTime() }, now), 'Next run Mon 30 Nov')
  assert.equal(runStatusEnd({ ...never, nextRunAt: now - minute }, now), 'Next run soon')
  assert.equal(runStatusEnd(never, now), 'Automatic runs off')
})
