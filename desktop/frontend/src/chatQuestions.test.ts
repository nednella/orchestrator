import assert from 'node:assert/strict'
import { test } from 'node:test'
import { answeredLate, approvalLine, firstOpenStep, isAnswered, keepPicks, keptPicks, pickedAnswer, questionAnswer, questionKey, questionsOf, toggledOther, withLabel, withOther } from './chatQuestions.ts'

const colour = { question: 'Which colour do you like?', header: 'Colour', options: [{ label: 'Red', description: 'Warm', preview: '' }, { label: 'Blue', description: 'Cool', preview: '' }], multiSelect: false }
const sizes = { question: 'Which sizes do you stock?', header: 'Sizes', options: [{ label: 'Small', description: '', preview: '' }, { label: 'Medium', description: '', preview: '' }, { label: 'Large', description: '', preview: '' }], multiSelect: true }

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
  const p = { id: 'p/a', request: 'r1', tool: 'Bash', summary: 'npm install', kind: 'tool' as const, input: { command: 'npm install' }, inputTooLarge: false }
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

test('an option keeps its preview, and the answer keeps it in the questions it sends back', () => {
  const layout = { question: 'Which layout?', header: 'Layout', options: [{ label: 'Stepper', description: 'Three routes', preview: '/a\n/b' }, { label: 'Single', description: '' }], multiSelect: false }
  assert.deepEqual(questionsOf({ questions: [layout] })[0].options.map((o) => o.preview), ['/a\n/b', ''])
  const sent = JSON.parse(questionAnswer({ questions: [layout] }, { [layout.question]: 'Stepper' }))
  assert.deepEqual(sent, { questions: [layout], answers: { 'Which layout?': 'Stepper' } })
})

test('a single choice takes one label, or the Other text', () => {
  const none = { labels: [], other: null }
  assert.deepEqual(withLabel(colour, none, 'Red'), { labels: ['Red'], other: null })
  assert.deepEqual(withLabel(colour, { labels: ['Red'], other: 'x' }, 'Blue'), { labels: ['Blue'], other: null })
  assert.deepEqual(withLabel(colour, { labels: ['Red'], other: null }, 'Red'), { labels: ['Red'], other: null }, 'a second click keeps the answer')
  assert.deepEqual(withOther(colour, { labels: ['Red'], other: null }, 'Green'), { labels: [], other: 'Green' })
  assert.deepEqual(toggledOther(colour, { labels: ['Red'], other: null }), { labels: [], other: '' })
  assert.deepEqual(toggledOther(colour, { labels: [], other: 'Green' }), { labels: [], other: 'Green' })
})

test('a multiple choice toggles labels and keeps them beside Other', () => {
  let pick = { labels: [] as string[], other: null as string | null }
  pick = withLabel(sizes, pick, 'Small')
  pick = withLabel(sizes, pick, 'Large')
  assert.deepEqual(pick.labels, ['Small', 'Large'])
  pick = toggledOther(sizes, pick)
  assert.deepEqual(pick, { labels: ['Small', 'Large'], other: '' })
  pick = withOther(sizes, pick, 'Huge')
  assert.deepEqual(pickedAnswer(sizes, pick), ['Small', 'Large', 'Huge'])
  assert.deepEqual(toggledOther(sizes, pick), { labels: ['Small', 'Large'], other: null })
  assert.deepEqual(withLabel(sizes, pick, 'Small').labels, ['Large'])
})

test('a pick is an answer with a label or Other text that is not blank', () => {
  assert.equal(isAnswered({ labels: [], other: null }), false)
  assert.equal(isAnswered({ labels: [], other: '' }), false)
  assert.equal(isAnswered({ labels: [], other: '  ' }), false)
  assert.equal(isAnswered({ labels: [], other: 'x' }), true)
  assert.equal(isAnswered({ labels: ['Red'], other: '' }), true)
})

test('the form opens on the first question left open', () => {
  const open = { labels: [], other: null }
  const done = { labels: ['Red'], other: null }
  assert.equal(firstOpenStep([open, open]), 0)
  assert.equal(firstOpenStep([done, open]), 1)
  assert.equal(firstOpenStep([done, done]), 1, 'all answered: the last step, where Submit is')
  assert.equal(firstOpenStep([]), 0)
})

test('number keys pick options, the next one opens Other, and Enter goes on', () => {
  assert.deepEqual(questionKey('1', false, colour), { do: 'option', index: 0 })
  assert.deepEqual(questionKey('2', false, colour), { do: 'option', index: 1 })
  assert.deepEqual(questionKey('3', false, colour), { do: 'other' })
  assert.equal(questionKey('4', false, colour), null)
  assert.equal(questionKey('0', false, colour), null)
  assert.equal(questionKey('a', false, colour), null)
  assert.equal(questionKey('1', true, colour), null, 'a digit typed into the Other field is text')
  assert.deepEqual(questionKey('Enter', false, colour), { do: 'next' })
  assert.deepEqual(questionKey('Enter', true, colour), { do: 'next' })
  assert.equal(questionKey('Escape', false, colour), null)
})
