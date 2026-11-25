import type { Pending } from './types'

export type QuestionOption = { label: string; description: string }
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
      const options = Array.isArray(i.options) ? i.options.map((o) => ({ label: str(asInput(o).label), description: str(asInput(o).description) })).filter((o) => o.label !== '') : []
      return { question: str(i.question), header: str(i.header), options, multiSelect: i.multiSelect === true }
    })
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
