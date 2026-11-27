import assert from 'node:assert/strict'
import { test } from 'node:test'
import { EFFORTS, effortsOf, optionsAfterModel, supportsFast } from './model.ts'

test('fast mode is offered for the models Claude gives it, by alias or full id', () => {
  assert.equal(supportsFast('opus'), true)
  assert.equal(supportsFast('claude-fable-5-2'), true)
  assert.equal(supportsFast('claude-opus-5-5[1m]'), true)
  assert.equal(supportsFast('sonnet'), false)
  assert.equal(supportsFast('haiku'), false)
  assert.equal(supportsFast(''), false)
  assert.equal(supportsFast('opusplan'), false)
})

test('ultracode comes after max, for the models that have it', () => {
  assert.deepEqual(effortsOf('opus'), ['low', 'medium', 'high', 'xhigh', 'max', 'ultracode'])
  assert.equal(EFFORTS.ultracode, 'Ultracode')
  assert.deepEqual(effortsOf('some-other-model'), ['low', 'medium', 'high', 'xhigh', 'max'])
})

test('a model without an option turns it off', () => {
  assert.deepEqual(optionsAfterModel('sonnet', true, 'high'), { fast: false, effort: 'high' })
  assert.deepEqual(optionsAfterModel('opus', true, 'ultracode'), { fast: true, effort: 'ultracode' })
  assert.deepEqual(optionsAfterModel('some-other-model', false, 'ultracode'), { fast: false, effort: 'xhigh' })
})

test('a model change keeps the default effort', () => {
  assert.deepEqual(optionsAfterModel('haiku', false, ''), { fast: false, effort: '' })
})
