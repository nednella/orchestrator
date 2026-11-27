import { Check, Minus } from 'lucide-react'
import { memo } from 'react'
import { questionsOf } from '../chatQuestions'
import type { Question } from '../chatQuestions'
import { OpenQuestions } from './OpenQuestions'

// id is the question's tool call; request is '' until Claude asks for the approval that answers the questions; answers
// is what they were answered with.
type ChatQuestionCardProps = { thread: string; id: string; request: string; input: unknown; answer: 'allow' | 'deny' | undefined; answers: Record<string, string> | null; expired: boolean; result: string | null }

export const ChatQuestionCard = memo(function ChatQuestionCard({ thread, id, request, input, answer, answers, expired, result }: ChatQuestionCardProps) {
  const questions = questionsOf(input)
  if (!answer && !expired && result === null) return <OpenQuestions thread={thread} id={id} request={request} input={input} questions={questions} />
  const status = answer === 'deny' ? 'Not answered' : expired ? 'The question expired' : 'Answered'
  const Mark = answer === 'deny' || expired ? Minus : Check

  return (
    <div className="chat-card q-done">
      <p className="q-done-head" data-answered={Mark === Check}>
        <Mark aria-hidden="true" />
        {status}
      </p>
      {answers ? (
        <div className="q-pills">
          {questions.map((q, i) => (
            <span key={q.question} className="q-pill select-text">
              <span>{q.header || `Question ${i + 1}`} →</span> {answers[q.question] ?? ''}
            </span>
          ))}
        </div>
      ) : (
        questions.map((q) => <QuestionTitle key={q.question} question={q} />)
      )}
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
