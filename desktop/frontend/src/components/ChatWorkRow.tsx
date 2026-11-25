import { ChevronDown, ChevronRight } from 'lucide-react'
import { memo, useLayoutEffect, useRef, useState } from 'react'
import { isOpened, setOpened } from '../chatOpen'
import { formatWorked } from '../chatTools'
import type { WorkStep } from '../chatTimeline'
import { onTick } from '../time'
import { ChatMarkdown } from './ChatMarkdown'
import { ChatThoughtRow } from './ChatThoughtRow'
import { ChatToolRow } from './ChatToolRow'

type ChatWorkRowProps = { thread: string; turn: string; header: boolean; steps: WorkStep[]; startedAt: number; endedAt: number | null; folder: string }

const sameStep = (a: WorkStep, b: WorkStep) => {
  if (a.kind === 'note') return b.kind === 'note' && a.text === b.text
  if (a.kind === 'thought') return b.kind === 'thought' && a.key === b.key && a.text === b.text && a.live === b.live
  return b.kind === 'call' && a.call.id === b.call.id && a.call.tool === b.call.tool && a.call.input === b.call.input && a.call.result === b.call.result && a.call.error === b.call.error && a.call.answer === b.call.answer
}

// The timeline is built anew on every streamed delta, so its steps are new objects holding the same values.
const unchanged = (a: ChatWorkRowProps, b: ChatWorkRowProps) =>
  a.thread === b.thread && a.turn === b.turn && a.header === b.header && a.startedAt === b.startedAt && a.endedAt === b.endedAt && a.folder === b.folder && a.steps.length === b.steps.length && a.steps.every((s, i) => sameStep(s, b.steps[i]))

export const ChatWorkRow = memo(function ChatWorkRow(props: ChatWorkRowProps) {
  const { thread, turn, header, steps, startedAt, endedAt, folder } = props
  if (endedAt !== null) return <FinishedWork thread={thread} turn={turn} steps={steps} startedAt={startedAt} worked={endedAt - startedAt} folder={folder} />
  return (
    <div>
      {header && <WorkingFor startedAt={startedAt} />}
      <Steps thread={thread} steps={steps} running folder={folder} />
    </div>
  )
}, unchanged)

type StepsProps = { thread: string; steps: WorkStep[]; running: boolean; folder: string }

function Steps({ thread, steps, running, folder }: StepsProps) {
  return (
    <>
      {steps.map((s) => {
        if (s.kind === 'note')
          return (
            <div key={s.key} className="py-1 text-small text-soft">
              <ChatMarkdown text={s.text} />
            </div>
          )
        if (s.kind === 'thought') return <ChatThoughtRow key={s.key} text={s.text} live={s.live} />
        return <ChatToolRow key={s.call.id} thread={thread} id={s.call.id} tool={s.call.tool} input={s.call.input} result={s.call.result} error={s.call.error} denied={s.call.answer === 'deny'} running={running} folder={folder} />
      })}
    </>
  )
}

type FinishedWorkProps = { thread: string; turn: string; steps: WorkStep[]; startedAt: number; worked: number; folder: string }

function FinishedWork({ thread, turn, steps, startedAt, worked, folder }: FinishedWorkProps) {
  const [open, setOpen] = useState(() => isOpened(thread, turn))
  const Chevron = open ? ChevronDown : ChevronRight
  return (
    <div>
      <div className="group chat-work-rule pr-0.5">
        <button
          className="flex items-center gap-1 rounded-md px-1 text-body leading-relaxed text-soft tabular-nums transition-colors duration-150 hover:text-ink"
          aria-expanded={open}
          onClick={() => {
            setOpened(thread, turn, !open)
            setOpen(!open)
          }}
        >
          Worked for {formatWorked(worked, true)}
          <Chevron size={14} aria-hidden />
        </button>
        <span className="ml-auto text-small text-soft tabular-nums opacity-0 group-focus-within:opacity-100 group-hover:opacity-100">
          {new Date(startedAt).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}
        </span>
      </div>
      {open && <Steps thread={thread} steps={steps} running={false} folder={folder} />}
    </div>
  )
}

type WorkingForProps = { startedAt: number }

// WorkingFor writes its time straight into the page at each tick of the clock, so the timeline never renders for it.
function WorkingFor({ startedAt }: WorkingForProps) {
  const time = useRef<HTMLSpanElement>(null)
  useLayoutEffect(() => {
    const show = (now: number) => {
      if (time.current) time.current.textContent = formatWorked(now - startedAt)
    }
    show(Date.now())
    return onTick(show)
  }, [startedAt])
  return (
    <div className="chat-work-rule">
      <p className="flex h-6 items-center px-1 text-body leading-relaxed text-soft tabular-nums">
        <span>
          Working for <span ref={time} />
        </span>
      </p>
    </div>
  )
}
