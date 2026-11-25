export type QuestionOption = { label: string; description: string }
export type Question = { question: string; header: string; options: QuestionOption[]; multiSelect: boolean }

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
// the request was answered elsewhere.
export const answeredLate = (message: string) => /^No approval .* is waiting$/.test(message)
