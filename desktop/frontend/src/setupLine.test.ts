import assert from 'node:assert/strict'
import { test } from 'node:test'
import { buildTimeline } from './chatTimeline.ts'
import { withSetup } from './setupLine.ts'

const timeline = buildTimeline([{ seq: 1, at: 1, kind: 'user', id: 'u', text: 'Fix it' }], {})

test('a thread with no set-up command keeps its chat as it is', () => {
  assert.equal(withSetup(timeline, { setup: '', setupExit: 0, setupShell: '' }), timeline)
})

test('a running set-up command is a quiet line after the working row, and the turn goes on', () => {
  const shown = withSetup(timeline, { setup: 'running', setupExit: 0, setupShell: 'shop/s1' })
  const last = shown.items.at(-1)
  assert.deepEqual(last, { kind: 'setup', key: 'setup', text: 'Running the set-up command', failed: false, shell: 'shop/s1' })
  assert.equal(shown.turnOpen, timeline.turnOpen)
})

test('a failed one says its exit code and names its terminal', () => {
  const last = withSetup(timeline, { setup: 'failed', setupExit: 3, setupShell: 'shop/s1' }).items.at(-1)
  assert.deepEqual(last, { kind: 'setup', key: 'setup', text: 'The set-up command exited with code 3', failed: true, shell: 'shop/s1' })
})
