import assert from 'node:assert/strict'
import { test } from 'node:test'
import { thoughtLine } from './chatThought.ts'

test('a thought shows its bold heading', () => {
  assert.equal(thoughtLine('**Planning the `checkout` route**\n\nI need to read the router first.'), 'Planning the checkout route')
  assert.equal(thoughtLine('  **Checking\nthe tests**'), 'Checking the tests')
})

test('without a heading, a thought shows its first sentence', () => {
  assert.equal(thoughtLine('I need to read notes.txt first. Then I can answer.'), 'I need to read notes.txt first.')
  assert.equal(thoughtLine('Is it the [router](src/routes.tsx)? It may be.'), 'Is it the router?')
  assert.equal(thoughtLine('先读文件。然后回答'), '先读文件。')
  assert.equal(thoughtLine('## Next step\nRun *all* the tests'), 'Next step Run all the tests')
})

test('a streaming thought grows until its first sentence ends', () => {
  assert.equal(thoughtLine('I need to read notes.tx'), 'I need to read notes.tx')
  assert.equal(thoughtLine('**Plann'), 'Plann')
  assert.equal(thoughtLine('I need to read notes.txt first so I can explain. I'), 'I need to read notes.txt first so I can explain.')
  assert.equal(thoughtLine(''), '')
})
