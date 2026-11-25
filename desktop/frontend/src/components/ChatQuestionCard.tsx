import { memo, useId, useState } from 'react'
import { useAgentos } from '../AgentosContext'
import { questionAnswer, questionsOf } from '../chatQuestions'
import type { Question } from '../chatQuestions'
import { useAnswering } from '../useAnswering'

// request is '' until Claude asks for the approval that answers the questions; answers is what they were answered with.
type ChatQuestionCardProps = { thread: string; request: string; input: unknown; answer: 'allow' | 'deny' | undefined; answers: Record<string, string> | null; expired: boolean; result: string | null }

export const ChatQuestionCard = memo(function ChatQuestionCard({ thread, request, input, answer, answers, expired, result }: ChatQuestionCardProps) {
  const questions = questionsOf(input)
  if (!answer && !expired && result === null) return <OpenQuestions thread={thread} request={request} input={input} questions={questions} />
  const status = answer === 'deny' ? 'Not answered' : expired ? 'The question expired' : answers ? '' : 'Answered'

  return (
    <div className="chat-card">
      {questions.map((q) => (
        <div key={q.question} className="flex flex-col gap-1">
          <QuestionTitle question={q} />
          {answers?.[q.question] && <p className="text-body break-words text-ink select-text">{answers[q.question]}</p>}
        </div>
      ))}
      {status && <p className="text-small text-dim">{status}</p>}
    </div>
  )
})

type QuestionTitleProps = { question: Question }

function QuestionTitle({ question }: QuestionTitleProps) {
  return (
    <div className="flex flex-col gap-0.5">
      {question.header && <p className="label">{question.header}</p>}
      <p className="text-body break-words text-soft select-text">{question.question}</p>
    </div>
  )
}

// A pick is the labels picked for one question, and the text typed for Other, null while Other is not picked.
type Pick = { labels: string[]; other: string | null }

const given = (question: Question, { labels, other }: Pick): string[] | string => {
  if (question.multiSelect) return other === null ? labels : [...labels, other]
  return other ?? labels[0] ?? ''
}

type OpenQuestionsProps = { thread: string; request: string; input: unknown; questions: Question[] }

function OpenQuestions({ thread, request, input, questions }: OpenQuestionsProps) {
  const a = useAgentos('answerQuestion')
  const { busy, send } = useAnswering()
  const [picks, setPicks] = useState<Pick[]>(() => questions.map(() => ({ labels: [], other: null })))

  const setPick = (i: number, pick: Pick) => setPicks((list) => list.map((p, n) => (n === i ? pick : p)))

  const submit = () => {
    if (busy || request === '') return
    const answers = Object.fromEntries(questions.map((q, i) => [q.question, given(q, picks[i])]))
    send(() => a.answerQuestion(thread, request, questionAnswer(input, answers)))
  }

  return (
    <div className="chat-card" data-waiting="true">
      {questions.map((q, i) => (
        <QuestionPicker key={q.question} question={q} pick={picks[i]} onPick={(pick) => setPick(i, pick)} onSubmit={submit} />
      ))}
      <div className="flex">
        <button className="btn btn-primary" disabled={busy || request === ''} onClick={submit}>
          Submit
        </button>
      </div>
    </div>
  )
}

type QuestionPickerProps = { question: Question; pick: Pick; onPick(pick: Pick): void; onSubmit(): void }

function QuestionPicker({ question, pick, onPick, onSubmit }: QuestionPickerProps) {
  const name = useId()
  const multi = question.multiSelect
  const type = multi ? 'checkbox' : 'radio'

  const toggle = (label: string) => {
    if (!multi) return onPick({ labels: [label], other: null })
    onPick({ ...pick, labels: pick.labels.includes(label) ? pick.labels.filter((l) => l !== label) : [...pick.labels, label] })
  }
  const pickOther = (text: string | null) => onPick(multi ? { ...pick, other: text } : { labels: [], other: text })

  return (
    <fieldset className="flex min-w-0 flex-col gap-2">
      <QuestionTitle question={question} />
      <div className="flex flex-col gap-1.5">
        {question.options.map((o) => (
          <label key={o.label} className="flex cursor-pointer items-start gap-2 text-body">
            <input type={type} name={name} checked={pick.labels.includes(o.label)} onChange={() => toggle(o.label)} />
            <span className="min-w-0">
              <span className="text-ink">{o.label}</span>
              {o.description && <span className="block text-small text-dim">{o.description}</span>}
            </span>
          </label>
        ))}
        <label className="flex cursor-pointer items-center gap-2 text-body">
          <input type={type} name={name} className="mt-0" checked={pick.other !== null} onChange={() => pickOther(multi && pick.other !== null ? null : (pick.other ?? ''))} />
          <span className="flex-none text-ink">Other</span>
          <input
            className="field h-7 min-w-0 flex-1 text-small"
            placeholder="Type your own answer"
            value={pick.other ?? ''}
            onChange={(e) => pickOther(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') onSubmit()
            }}
          />
        </label>
      </div>
    </fieldset>
  )
}
