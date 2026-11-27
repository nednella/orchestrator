import assert from 'node:assert/strict'
import { test } from 'node:test'
import { effortPick, firstTab, listedModels, reasoningChosen, reasoningLevels, toggled, ultracodeEffort } from './modelMenu.ts'

test('the Claude tab lists every model with the starred ones first', () => {
  assert.deepEqual(listedModels('claude', '', []), ['fable', 'opus', 'sonnet', 'haiku'])
  assert.deepEqual(listedModels('claude', '', ['haiku', 'opus']), ['opus', 'haiku', 'fable', 'sonnet'])
})

test('the Favourites tab lists only the starred models, and a search narrows them', () => {
  assert.deepEqual(listedModels('favorites', '', ['sonnet']), ['sonnet'])
  assert.deepEqual(listedModels('favorites', '', []), [])
  assert.deepEqual(listedModels('favorites', 'son', ['haiku', 'sonnet']), ['sonnet'])
})

test('a search matches the name without regard to case', () => {
  assert.deepEqual(listedModels('claude', 'OPUS', []), ['opus'])
  assert.deepEqual(listedModels('claude', ' 5.5 ', []), ['opus', 'sonnet', 'haiku'])
  assert.deepEqual(listedModels('claude', 'zzz', []), [])
  assert.deepEqual(listedModels('claude', 'sonnet', []), ['sonnet'])
})

test('the menu opens on Favourites only when a starred model is on offer', () => {
  assert.equal(firstTab([]), 'claude')
  assert.equal(firstTab(['gone']), 'claude')
  assert.equal(firstTab(['haiku']), 'favorites')
})

test('toggling a star adds the model last or takes it out', () => {
  assert.deepEqual(toggled(['opus'], 'haiku'), ['opus', 'haiku'])
  assert.deepEqual(toggled(['opus', 'haiku'], 'opus'), ['haiku'])
})

test('the Reasoning column leaves ultracode to its toggle and shows xhigh while it is on', () => {
  assert.deepEqual(reasoningLevels('opus'), ['low', 'medium', 'high', 'xhigh', 'max'])
  assert.equal(reasoningChosen('ultracode'), 'xhigh')
  assert.equal(reasoningChosen('high'), 'high')
})

test('an effort pick that is the default is saved as the empty choice', () => {
  assert.equal(effortPick('medium', 'medium'), '')
  assert.equal(effortPick('high', 'medium'), 'high')
  assert.equal(ultracodeEffort(true), 'ultracode')
  assert.equal(ultracodeEffort(false), 'xhigh')
})
