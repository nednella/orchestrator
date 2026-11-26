import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { UpdateState } from './types.ts'
import { installConfirm, updateAction, updateLook } from './updateButton.ts'

const state = (s: Partial<UpdateState>): UpdateState => ({ status: 'idle', version: '', percent: 0, error: '', ...s })
const plain = { spin: false, dot: false, ring: null, success: false }

test('idle checks for updates', () => {
  assert.deepEqual(updateLook(state({})), { ...plain, icon: 'refresh-cw', tooltip: 'Check for updates' })
  assert.equal(updateAction(state({})), 'check')
})

test('checking spins and waits', () => {
  assert.deepEqual(updateLook(state({ status: 'checking' })), { ...plain, icon: 'refresh-cw', spin: true, tooltip: 'Checking for updates…' })
  assert.equal(updateAction(state({ status: 'checking' })), null)
})

test('available is green with a dot and downloads', () => {
  const s = state({ status: 'available', version: '1.4.0' })
  assert.deepEqual(updateLook(s), { ...plain, icon: 'download', dot: true, success: true, tooltip: 'Update 1.4.0 ready to download' })
  assert.equal(updateAction(s), 'download')
})

test('downloading shows the progress in a ring and waits', () => {
  const s = state({ status: 'downloading', version: '1.4.0', percent: 40 })
  assert.deepEqual(updateLook(s), { ...plain, icon: 'download', ring: 40, tooltip: 'Downloading update (40%)' })
  assert.equal(updateAction(s), null)
})

test('the percent is rounded down', () => {
  const look = updateLook(state({ status: 'downloading', version: '1.4.0', percent: 69.9 }))
  assert.equal(look.ring, 69)
  assert.equal(look.tooltip, 'Downloading update (69%)')
})

test('a download of unknown size has no ring', () => {
  assert.deepEqual(updateLook(state({ status: 'downloading', version: '1.4.0', percent: -1 })), { ...plain, icon: 'download', tooltip: 'Downloading update…' })
})

test('downloaded has the check badge and installs', () => {
  const s = state({ status: 'downloaded', version: '1.4.0', percent: 100 })
  assert.deepEqual(updateLook(s), { ...plain, icon: 'rotate-cw', dot: true, tooltip: 'Update 1.4.0 downloaded. Click to restart and install.' })
  assert.equal(updateAction(s), 'install')
})

test('a failed download retries the download', () => {
  const s = state({ status: 'failed', version: '1.4.0', percent: 40, error: 'curl: (18)' })
  assert.deepEqual(updateLook(s), { ...plain, icon: 'download', tooltip: 'Download failed for 1.4.0. Click to retry.' })
  assert.equal(updateAction(s), 'download')
})

test('a failed install retries the install', () => {
  const s = state({ status: 'failed', version: '1.4.0', percent: 100, error: 'sha256' })
  assert.deepEqual(updateLook(s), { ...plain, icon: 'rotate-cw', tooltip: 'Install failed for 1.4.0. Click to retry.' })
  assert.equal(updateAction(s), 'install')
})

test('the install asks to restart', () => {
  assert.deepEqual(installConfirm('1.4.0'), { title: 'Install update 1.4.0 and restart the app?', body: ['Running threads keep going: their hosts outlive the app.'], confirm: 'Restart', tone: 'primary' })
})
