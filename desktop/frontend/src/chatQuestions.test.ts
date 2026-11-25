import assert from 'node:assert/strict'
import { test } from 'node:test'
import { answeredLate, approvalLine, keepPicks, keptPicks, pickedAnswer, questionAnswer, questionsOf } from './chatQuestions.ts'

const colour = { question: 'Which colour do you like?', header: 'Colour', options: [{ label: 'Red', description: 'Warm' }, { label: 'Blue', description: 'Cool' }], multiSelect: false }
const sizes = { question: 'Which sizes do you stock?', header: 'Sizes', options: [{ label: 'Small', description: '' }, { label: 'Medium', description: '' }, { label: 'Large', description: '' }], multiSelect: true }

test('single pick answer shape', () => {
  const input = { questions: [colour] }
  assert.deepEqual(JSON.parse(questionAnswer(input, { [colour.question]: 'Red' })), { questions: [colour], answers: { 'Which colour do you like?': 'Red' } })
})

test('multi-select joins labels', () => {
  const answer = JSON.parse(questionAnswer({ questions: [colour, sizes] }, { [colour.question]: 'Blue', [sizes.question]: ['Small', 'Large'] }))
  assert.deepEqual(answer.answers, { 'Which colour do you like?': 'Blue', 'Which sizes do you stock?': 'Small, Large' })
})

test('other text is used', () => {
  const answer = JSON.parse(questionAnswer({ questions: [colour, sizes] }, { [colour.question]: 'Green, mostly', [sizes.question]: ['Small', 'Extra large'] }))
  assert.deepEqual(answer.answers, { 'Which colour do you like?': 'Green, mostly', 'Which sizes do you stock?': 'Small, Extra large' })
})

test('missing pick throws', () => {
  const input = { questions: [colour, sizes] }
  assert.throws(() => questionAnswer(input, { [colour.question]: 'Red' }), /^Answer every question$/)
  assert.throws(() => questionAnswer(input, { [colour.question]: 'Red', [sizes.question]: [] }), /^Answer every question$/)
  assert.throws(() => questionAnswer(input, { [colour.question]: '  ', [sizes.question]: ['Small'] }), /^Answer every question$/)
})

test('questions read from the tool input', () => {
  assert.deepEqual(questionsOf({ questions: [colour, { question: 'Why?' }, 'junk'] }), [colour, { question: 'Why?', header: '', options: [], multiSelect: false }])
  assert.deepEqual(questionsOf(null), [])
})

test('a refused answer to an approval that ended', () => {
  assert.equal(answeredLate('No approval "req-1" is waiting'), true)
  assert.equal(answeredLate('Thread storefront/abcd2345 did not take the answer'), false)
})

test('a tool approval reads as its tool and summary', () => {
  const p = { id: 'p/a', request: 'r1', tool: 'Bash', summary: 'npm install', kind: 'tool' as const }
  assert.equal(approvalLine(p), 'Bash: npm install')
  assert.equal(approvalLine({ ...p, tool: 'mcp__browser__click', summary: '' }), 'mcp__browser__click')
})

test('a pick gives the labels and the Other text', () => {
  assert.equal(pickedAnswer(colour, { labels: ['Red'], other: null }), 'Red')
  assert.equal(pickedAnswer(colour, { labels: [], other: 'Green' }), 'Green')
  assert.deepEqual(pickedAnswer(sizes, { labels: ['Small'], other: 'Huge' }), ['Small', 'Huge'])
})

test('picks are kept per question, for at most 20 questions', () => {
  keepPicks('p/a', 'toolu_1', [{ labels: ['Red'], other: null }])
  assert.deepEqual(keptPicks('p/a', 'toolu_1'), [{ labels: ['Red'], other: null }])
  assert.equal(keptPicks('p/b', 'toolu_1'), undefined)
  for (let i = 0; i < 20; i++) keepPicks('p/c', `toolu_${i}`, [])
  assert.equal(keptPicks('p/a', 'toolu_1'), undefined)
  assert.deepEqual(keptPicks('p/c', 'toolu_19'), [])
})
