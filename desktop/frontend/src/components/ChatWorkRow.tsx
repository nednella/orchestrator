import { memo, useState } from 'react'
import { formatWorked } from '../chatTools'
import type { ToolCall } from '../chatTimeline'
import { useClock } from '../time'
import { ChatToolRow } from './ChatToolRow'
import { Icon } from './Icon'

type ChatWorkRowProps = { calls: ToolCall[]; startedAt: number; endedAt: number | null; ticking: boolean; folder: string }

const sameCall = (a: ToolCall, b: ToolCall) => a.id === b.id && a.tool === b.tool && a.input === b.input && a.result === b.result && a.error === b.error

// The timeline is built anew on every streamed delta, so its calls are new objects holding the same values.
const unchanged = (a: ChatWorkRowProps, b: ChatWorkRowProps) =>
  a.startedAt === b.startedAt && a.endedAt === b.endedAt && a.ticking === b.ticking && a.folder === b.folder && a.calls.length === b.calls.length && a.calls.every((c, i) => sameCall(c, b.calls[i]))

export const ChatWorkRow = memo(function ChatWorkRow({ calls, startedAt, endedAt, ticking, folder }: ChatWorkRowProps) {
  if (endedAt === null) return <RunningWork calls={calls} startedAt={startedAt} ticking={ticking} folder={folder} />
  return <FinishedWork calls={calls} worked={endedAt - startedAt} folder={folder} />
}, unchanged)

type FinishedWorkProps = { calls: ToolCall[]; worked: number; folder: string }

function FinishedWork({ calls, worked, folder }: FinishedWorkProps) {
  const [open, setOpen] = useState(false)
  return (
    <div>
      <button className="flex items-center gap-1.5 py-1 text-small text-dim hover:text-soft" aria-expanded={open} onClick={() => setOpen(!open)}>
        <span className={`flex transition-transform ${open ? '' : '-rotate-90'}`}>
          <Icon name="chevron" size={12} />
        </span>
        Worked for {formatWorked(worked)} · {calls.length} tool {calls.length === 1 ? 'call' : 'calls'}
      </button>
      {open && (
        <div className="ml-1.5 border-l border-line pl-3">
          {calls.map((c) => (
            <ChatToolRow key={c.id} tool={c.tool} input={c.input} result={c.result} error={c.error} running={false} folder={folder} />
          ))}
        </div>
      )}
    </div>
  )
}

type RunningWorkProps = { calls: ToolCall[]; startedAt: number; ticking: boolean; folder: string }

function RunningWork({ calls, startedAt, ticking, folder }: RunningWorkProps) {
  return (
    <div>
      {ticking && <WorkingFor startedAt={startedAt} />}
      {calls.map((c) => (
        <ChatToolRow key={c.id} tool={c.tool} input={c.input} result={c.result} error={c.error} running folder={folder} />
      ))}
    </div>
  )
}

type WorkingForProps = { startedAt: number }

function WorkingFor({ startedAt }: WorkingForProps) {
  const text = useClock((now) => formatWorked(now - startedAt))
  return <p className="py-1 text-small text-dim">Working for {text}</p>
}
