import { memo, useState } from 'react'
import { isOpened, setOpened } from '../chatOpen'
import { formatWorked } from '../chatTools'
import type { WorkStep } from '../chatTimeline'
import { useClock } from '../time'
import { ChatMarkdown } from './ChatMarkdown'
import { ChatToolRow } from './ChatToolRow'
import { Icon } from './Icon'

type ChatWorkRowProps = { thread: string; turn: string; steps: WorkStep[]; startedAt: number; endedAt: number | null; folder: string }

const sameStep = (a: WorkStep, b: WorkStep) => {
  if (a.kind === 'note') return b.kind === 'note' && a.text === b.text
  return b.kind === 'call' && a.call.id === b.call.id && a.call.tool === b.call.tool && a.call.input === b.call.input && a.call.result === b.call.result && a.call.error === b.call.error
}

// The timeline is built anew on every streamed delta, so its steps are new objects holding the same values.
const unchanged = (a: ChatWorkRowProps, b: ChatWorkRowProps) =>
  a.thread === b.thread && a.turn === b.turn && a.startedAt === b.startedAt && a.endedAt === b.endedAt && a.folder === b.folder && a.steps.length === b.steps.length && a.steps.every((s, i) => sameStep(s, b.steps[i]))

export const ChatWorkRow = memo(function ChatWorkRow(props: ChatWorkRowProps) {
  const { thread, turn, steps, startedAt, endedAt, folder } = props
  if (endedAt === null) return <RunningWork thread={thread} steps={steps} startedAt={startedAt} folder={folder} />
  return <FinishedWork thread={thread} turn={turn} steps={steps} worked={endedAt - startedAt} folder={folder} />
}, unchanged)

type StepsProps = { thread: string; steps: WorkStep[]; running: boolean; folder: string }

function Steps({ thread, steps, running, folder }: StepsProps) {
  return (
    <>
      {steps.map((s) =>
        s.kind === 'note' ? (
          <div key={s.key} className="py-1 text-small text-soft">
            <ChatMarkdown text={s.text} />
          </div>
        ) : (
          <ChatToolRow key={s.call.id} thread={thread} id={s.call.id} tool={s.call.tool} input={s.call.input} result={s.call.result} error={s.call.error} running={running} folder={folder} />
        ),
      )}
    </>
  )
}

type FinishedWorkProps = { thread: string; turn: string; steps: WorkStep[]; worked: number; folder: string }

function FinishedWork({ thread, turn, steps, worked, folder }: FinishedWorkProps) {
  const [open, setOpen] = useState(() => isOpened(thread, turn))
  const calls = steps.filter((s) => s.kind === 'call').length
  return (
    <div>
      <button
        className="flex items-center gap-1.5 py-1 text-small text-dim hover:text-soft"
        aria-expanded={open}
        onClick={() => {
          setOpened(thread, turn, !open)
          setOpen(!open)
        }}
      >
        <span className={`flex transition-transform ${open ? '' : '-rotate-90'}`}>
          <Icon name="chevron" size={12} />
        </span>
        Worked for {formatWorked(worked)} · {calls} tool {calls === 1 ? 'call' : 'calls'}
      </button>
      {open && (
        <div className="ml-1.5 border-l border-line pl-3">
          <Steps thread={thread} steps={steps} running={false} folder={folder} />
        </div>
      )}
    </div>
  )
}

type RunningWorkProps = { thread: string; steps: WorkStep[]; startedAt: number; folder: string }

function RunningWork({ thread, steps, startedAt, folder }: RunningWorkProps) {
  return (
    <div>
      <WorkingFor startedAt={startedAt} />
      <Steps thread={thread} steps={steps} running folder={folder} />
    </div>
  )
}

type WorkingForProps = { startedAt: number }

function WorkingFor({ startedAt }: WorkingForProps) {
  const text = useClock((now) => formatWorked(now - startedAt))
  return <p className="py-1 text-small text-dim">Working for {text}</p>
}
