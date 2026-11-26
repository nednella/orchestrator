import { memo } from 'react'
import { WorkingTime } from './WorkingTime'

type ChatWorkingRowProps = { startedAt: number }

// ChatWorkingRow heads the turn that runs with how long it has worked.
export const ChatWorkingRow = memo(function ChatWorkingRow({ startedAt }: ChatWorkingRowProps) {
  return (
    <div className="chat-work-rule">
      <p className="flex h-6 items-center px-1 text-body leading-relaxed text-soft tabular-nums">
        <span>
          Working for <WorkingTime since={startedAt} />
        </span>
      </p>
    </div>
  )
})
