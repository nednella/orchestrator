import { ListEnd } from 'lucide-react'
import { firstLine } from '../chatTimeline'
import type { QueuedMessage } from '../chatTimeline'

// Messages Claude has not taken in yet. Send now stops the turn so Claude takes them in at once.
type ChatQueuedProps = { queued: QueuedMessage[]; stopping: boolean; onSendNow(id: string): void }

export function ChatQueued({ queued, stopping, onSendNow }: ChatQueuedProps) {
  return (
    <ul className="flex flex-col" aria-label="Queued messages">
      {queued.map((m) => (
        <li key={m.id} className="flex min-w-0 items-center gap-2 py-0.5 pr-0.5 pl-2 text-small">
          <span className="flex flex-none text-dim" title="Queued: Claude takes it in at its next step">
            <ListEnd size={12} aria-hidden="true" />
          </span>
          <span className="min-w-0 flex-1 truncate text-soft">{firstLine(m.text)}</span>
          <button className="btn h-6 flex-none px-2 text-label" disabled={stopping} onMouseDown={(e) => e.preventDefault()} onClick={() => onSendNow(m.id)}>
            Send now
          </button>
        </li>
      ))}
    </ul>
  )
}
