import type { Pending } from './types'

// preview is the markdown Claude may give an option to show it by: an ASCII mockup, code, an image or an html page.
export type QuestionOption = { label: string; description: string; preview: string }
export type Question = { question: string; header: string; options: QuestionOption[]; multiSelect: boolean }
// A pick is the labels picked for one question, and the text typed for Other, null while Other is not picked.
export type Pick = { labels: string[]; other: string | null }

type Input = Record<string, unknown>

const asInput = (v: unknown): Input => (v && typeof v === 'object' ? (v as Input) : {})
const str = (v: unknown): string => (typeof v === 'string' ? v : '')

// questionsOf reads the questions of an AskUserQuestion call's input.
export function questionsOf(input: unknown): Question[] {
  const list = asInput(input).questions
  if (!Array.isArray(list)) return []
  return list
    .filter((q) => typeof asInput(q).question === 'string')
    .map((q) => {
      const i = asInput(q)
      const options = Array.isArray(i.options) ? i.options.map((o) => ({ label: str(asInput(o).label), description: str(asInput(o).description), preview: str(asInput(o).preview) })).filter((o) => o.label !== '') : []
      return { question: str(i.question), header: str(i.header), options, multiSelect: i.multiSelect === true }
    })
}

// isAnswered reports whether the pick holds an answer: a label, or text typed for Other.
export const isAnswered = ({ labels, other }: Pick): boolean => labels.length > 0 || (other !== null && other.trim() !== '')

// withLabel is the pick once the option with label is chosen: the one answer of a single choice, or the label toggled
// among those of a multiple choice.
export function withLabel(question: Question, pick: Pick, label: string): Pick {
  if (!question.multiSelect) return { labels: [label], other: null }
  return { ...pick, labels: pick.labels.includes(label) ? pick.labels.filter((l) => l !== label) : [...pick.labels, label] }
}

// withOther is the pick once Other is chosen with text, or, with null, taken off; a single choice drops its label.
export function withOther(question: Question, pick: Pick, text: string | null): Pick {
  return question.multiSelect ? { ...pick, other: text } : { labels: [], other: text }
}

// toggledOther is the pick once the Other tile is clicked: it opens an empty Other, or closes the open one of a
// multiple choice; a single choice keeps it open.
export function toggledOther(question: Question, pick: Pick): Pick {
  return withOther(question, pick, question.multiSelect && pick.other !== null ? null : (pick.other ?? ''))
}

// firstOpenStep is the first question the picks leave unanswered, else the last.
export function firstOpenStep(picks: Pick[]): number {
  const open = picks.findIndex((p) => !isAnswered(p))
  return open < 0 ? Math.max(0, picks.length - 1) : open
}

// QuestionKey is what a key does in a question's step: pick the option at index, open Other, or go on (Next, or Submit
// on the last step).
export type QuestionKey = { do: 'option'; index: number } | { do: 'other' } | { do: 'next' }

// questionKey reads a key pressed in a step of question. 1 to the number of options pick one, the number after them
// opens Other, and Enter goes on, also while typing the Other text. A digit typed into a field is text.
export function questionKey(key: string, typing: boolean, question: Question): QuestionKey | null {
  if (key === 'Enter') return { do: 'next' }
  if (typing || !/^[1-9]$/.test(key)) return null
  const n = Number(key)
  if (n <= question.options.length) return { do: 'option', index: n - 1 }
  return n === question.options.length + 1 ? { do: 'other' } : null
}

// pickedAnswer is a question's entry in questionAnswer's picks.
export function pickedAnswer(question: Question, { labels, other }: Pick): string[] | string {
  if (question.multiSelect) return other === null ? labels : [...labels, other]
  return other ?? labels[0] ?? ''
}

// The picks of questions not yet answered, kept outside the card so that they outlive it when the list lets go of
// its row. Keys are a thread and the question's tool call, which, unlike its approval, is known from the start.
const keptQuestions = 20
const picked = new Map<string, Pick[]>()

export const keptPicks = (thread: string, call: string): Pick[] | undefined => picked.get(`${thread} ${call}`)

// keepPicks marks the question as the newest one touched, and drops the oldest past the limit.
export function keepPicks(thread: string, call: string, picks: Pick[]) {
  const key = `${thread} ${call}`
  picked.delete(key)
  picked.set(key, picks)
  const oldest = picked.keys().next().value
  if (picked.size > keptQuestions && oldest !== undefined) picked.delete(oldest)
}

// questionAnswer is the input that allows an AskUserQuestion call: its own, plus each question's answer keyed by the
// question's text. picks holds, by question, the labels picked or the text typed for Other.
export function questionAnswer(input: unknown, picks: Record<string, string[] | string>): string {
  const answers: Record<string, string> = {}
  for (const { question } of questionsOf(input)) {
    const pick = picks[question] ?? []
    const given = (Array.isArray(pick) ? pick : [pick]).filter((p) => p.trim() !== '')
    if (given.length === 0) throw 'Answer every question'
    answers[question] = given.join(', ')
  }
  return JSON.stringify({ ...asInput(input), answers })
}

// answeredLate reports whether the host refused an answer because its approval no longer waits: the turn ended, or
// the request was answered elsewhere. The pattern is the host's sentence `no approval %q is waiting` (internal/host),
// as the chat service passes it on; desktop/chat/chat_test.go checks that sentence, so change both together.
export const answeredLate = (message: string) => /^No approval .* is waiting$/.test(message)

// approvalLine is what a thread's card says a tool approval would allow, from the approval itself.
export const approvalLine = (p: Pending): string => (p.summary ? `${p.tool}: ${p.summary}` : p.tool)
