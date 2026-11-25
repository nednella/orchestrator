import { ChevronDown, ChevronRight } from 'lucide-react'
import { memo, useLayoutEffect, useRef, useState } from 'react'
import { isOpened, setOpened } from '../chatOpen'
import { formatWorked } from '../chatTools'
import { sameSteps } from '../chatTimeline'
import type { WorkStep } from '../chatTimeline'
import { onTick } from '../time'
import { ChatSteps } from './ChatSteps'

type ChatWorkRowProps = { thread: string; turn: string; header: boolean; steps: WorkStep[]; startedAt: number; endedAt: number | null; folder: string }

// The timeline is built anew on every streamed delta, so its steps are new objects holding the same values.
const unchanged = (a: ChatWorkRowProps, b: ChatWorkRowProps) =>
  a.thread === b.thread && a.turn === b.turn && a.header === b.header && a.startedAt === b.startedAt && a.endedAt === b.endedAt && a.folder === b.folder && sameSteps(a.steps, b.steps)

export const ChatWorkRow = memo(function ChatWorkRow(props: ChatWorkRowProps) {
  const { thread, turn, header, steps, startedAt, endedAt, folder } = props
  if (endedAt !== null) return <FinishedWork thread={thread} turn={turn} steps={steps} startedAt={startedAt} worked={endedAt - startedAt} folder={folder} />
  return (
    <div>
      {header && <WorkingFor startedAt={startedAt} />}
      <ChatSteps thread={thread} steps={steps} running folder={folder} />
    </div>
  )
}, unchanged)

type FinishedWorkProps = { thread: string; turn: string; steps: WorkStep[]; startedAt: number; worked: number; folder: string }

// FinishedWork is the turn's heading once it ends, which opens its steps; a turn with none keeps it as a plain line.
function FinishedWork({ thread, turn, steps, startedAt, worked, folder }: FinishedWorkProps) {
  const [open, setOpen] = useState(() => isOpened(thread, turn))
  const Chevron = open ? ChevronDown : ChevronRight
  const label = `Worked for ${formatWorked(worked, true)}`
  return (
    <div>
      <div className="group chat-work-rule pr-0.5">
        {steps.length === 0 ? (
          <p className="flex h-6 items-center px-1 text-body leading-relaxed text-soft tabular-nums">{label}</p>
        ) : (
          <button
            className="flex h-6 items-center gap-1 rounded-md px-1 text-body leading-relaxed text-soft tabular-nums transition-colors duration-150 hover:text-ink"
            aria-expanded={open}
            onClick={() => {
              setOpened(thread, turn, !open)
              setOpen(!open)
            }}
          >
            {label}
            <Chevron size={14} aria-hidden />
          </button>
        )}
        <span className="ml-auto text-small text-soft tabular-nums opacity-0 group-focus-within:opacity-100 group-hover:opacity-100">
          {new Date(startedAt).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}
        </span>
      </div>
      {open && <ChatSteps thread={thread} steps={steps} running={false} folder={folder} />}
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
