import { useEffect, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { firstOpenStep, isAnswered, keepPicks, keptPicks, pickedAnswer, questionAnswer, questionKey, toggledOther, withLabel, withOther } from '../chatQuestions'
import type { Pick, Question } from '../chatQuestions'
import { useAnswering } from '../useAnswering'
import { QuestionOtherTile } from './QuestionOtherTile'
import { QuestionTile } from './QuestionTile'

type OpenQuestionsProps = { thread: string; id: string; request: string; input: unknown; questions: Question[] }

// OpenQuestions asks the questions one at a time, as a step each. The picks are kept, so that they outlive the card.
export function OpenQuestions({ thread, id, request, input, questions }: OpenQuestionsProps) {
  const a = useOrchestrator('answerQuestion')
  const { busy, send } = useAnswering()
  const [picks, setPicks] = useState<Pick[]>(() => keptPicks(thread, id) ?? questions.map(() => ({ labels: [], other: null })))
  const [step, setStep] = useState(() => Math.min(firstOpenStep(picks), questions.length - 1))
  const otherInput = useRef<HTMLInputElement>(null)
  const card = useRef<HTMLDivElement>(null)
  const shownStep = useRef(step)

  // A step taller than the chat leaves the chat at its end, below the question: bring the card's top into view.
  useEffect(() => {
    if (shownStep.current === step) return
    shownStep.current = step
    // The chat anchors to its end as the card is measured; wait for that.
    const wait = setTimeout(() => card.current?.scrollIntoView({ block: 'start', behavior: 'smooth' }), 150)
    return () => clearTimeout(wait)
  }, [step])

  const question = questions[step]
  const pick = picks[step]
  const last = step === questions.length - 1
  const many = questions.length > 1
  const allAnswered = picks.every(isAnswered)
  const canGoOn = isAnswered(pick) && (!last || (allAnswered && !busy && request !== ''))

  const setPick = (next: Pick) => {
    const all = picks.map((p, n) => (n === step ? next : p))
    keepPicks(thread, id, all)
    setPicks(all)
  }
  const openOther = () => {
    setPick(toggledOther(question, pick))
    // The input is not there until this render is done.
    setTimeout(() => otherInput.current?.focus())
  }

  const submit = () => {
    if (busy || request === '') return
    const answers = Object.fromEntries(questions.map((q, i) => [q.question, pickedAnswer(q, picks[i])]))
    send(() => a.answerQuestion(thread, request, questionAnswer(input, answers)))
  }
  const goOn = () => {
    if (!canGoOn) return
    if (last) submit()
    else setStep(step + 1)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.nativeEvent.isComposing || e.metaKey || e.ctrlKey || e.altKey || e.shiftKey || !question) return
    const target = e.target as HTMLElement
    // A button other than a tile does what Enter does to it.
    const button = target.closest('button')
    if (e.key === 'Enter' && button && !button.hasAttribute('data-q-tile')) return
    const key = questionKey(e.key, target instanceof HTMLInputElement, question)
    if (!key) return
    e.preventDefault()
    if (key.do === 'next') goOn()
    else if (key.do === 'other') openOther()
    else setPick(withLabel(question, pick, question.options[key.index].label))
  }

  if (!question) return null
  return (
    <div className="q-wrap">
      <div ref={card} className="q-card" data-waiting="true" role="group" aria-label={question.question} tabIndex={0} onKeyDown={onKeyDown}>
        <div className="q-head">
          <span className="dot q-dot" />
          {many ? <span className="q-step">{`Question ${step + 1} of ${questions.length}`}</span> : <span className="q-step">Claude needs your answer</span>}
          {question.header && <span className="q-header">{question.header}</span>}
          {many && (
            <div className="q-prog" aria-hidden="true">
              {questions.map((q, i) => (
                <span key={q.question} className="q-bar" data-state={i < step ? 'done' : i === step ? 'now' : 'next'} />
              ))}
            </div>
          )}
        </div>
        <div className="q-body">
          <div>
            <p className="q-title select-text">{question.question}</p>
            {question.multiSelect && <p className="q-hint">Pick any</p>}
          </div>
          <div className="q-tiles" role={question.multiSelect ? 'group' : 'radiogroup'} aria-label={question.question}>
            {question.options.map((o, i) => (
              <QuestionTile key={o.label} thread={thread} number={i + 1} option={o} multi={question.multiSelect} on={pick.labels.includes(o.label)} onPick={() => setPick(withLabel(question, pick, o.label))} />
            ))}
            <QuestionOtherTile number={question.options.length + 1} multi={question.multiSelect} text={pick.other} inputRef={otherInput} onToggle={openOther} onText={(text) => setPick(withOther(question, pick, text))} />
          </div>
        </div>
        <div className="q-foot">
          {many && (
            <button type="button" className="btn btn-ghost" disabled={step === 0} onClick={() => setStep(step - 1)}>
              Back
            </button>
          )}
          <div className="q-hints">
            <span>
              <kbd className="q-key">1</kbd>-<kbd className="q-key">{question.options.length + 1}</kbd> pick
            </span>
            <span>
              <kbd className="q-key">Enter</kbd> {last ? 'submit' : 'next'}
            </span>
          </div>
          <button type="button" className="btn btn-primary" disabled={!canGoOn} onClick={goOn}>
            {last ? 'Submit' : 'Next'}
          </button>
        </div>
      </div>
    </div>
  )
}
