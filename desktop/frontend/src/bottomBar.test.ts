import assert from 'node:assert/strict'
import { test } from 'node:test'
import { BAR_BUTTONS, barLayout } from './bottomBar.ts'

const ids = (buttons: readonly { id: string }[]) => buttons.map((b) => b.id)

// The six buttons and their gaps take 212 px, beside the 4 px gap, the 32 px update button and 8 px at each side.
const allFit = 212 + 4 + 32 + 16

test('every button shows when it fits', () => {
  assert.deepEqual(barLayout(allFit), { shown: [...BAR_BUTTONS], more: [] })
  assert.deepEqual(barLayout(400).more, [])
})

test('news goes into more first', () => {
  assert.deepEqual(ids(barLayout(allFit - 1).shown), ['settings', 'pulls', 'issues', 'usage'])
  assert.deepEqual(ids(barLayout(allFit - 1).more), ['terminal', 'news'])
  assert.deepEqual(ids(barLayout(172).shown), ['settings', 'pulls'])
  assert.deepEqual(ids(barLayout(172).more), ['issues', 'usage', 'terminal', 'news'])
})

test('the order is kept', () => {
  assert.deepEqual(ids(BAR_BUTTONS), ['settings', 'pulls', 'issues', 'usage', 'terminal', 'news'])
  for (const width of [0, 120, 160, 172, 208, allFit - 1, allFit]) {
    const { shown, more } = barLayout(width)
    assert.deepEqual(ids([...shown, ...more]), ids(BAR_BUTTONS), `at ${width} px`)
  }
})
