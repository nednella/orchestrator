import { LoaderCircle } from 'lucide-react'
import type { CSSProperties } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'
import { COLUMN_DEFAULT_REM } from '../chatColumn'
import { startStatus } from '../threadStart'
import type { Thread } from '../types'
import { ChatUserRow } from './ChatUserRow'

// ChatStarting is the chat of a thread whose agent has not started: the message it was started with, as sent, and how
// the start goes, or why it failed with a retry. The thread's chat takes its place once the agent runs.
type ChatStartingProps = { thread: Thread }

export function ChatStarting({ thread }: ChatStartingProps) {
  const { report } = useOrchestrator('report')
  const status = startStatus(thread)
  if (status.kind === 'none') return null
  return (
    <div data-chat-starting className="flex h-full min-h-0 flex-col bg-app">
      <div className="chat-scroll h-full overflow-y-auto pt-4">
        <div className="chat-column flex flex-col gap-6 pb-6" style={{ '--chat-column': `${COLUMN_DEFAULT_REM}rem` } as CSSProperties}>
          {thread.firstMessage !== '' && <ChatUserRow thread={thread.id} id={`${thread.id}/first`} uuid="" text={thread.firstMessage} files={[]} at={thread.createdAt} afterUser={false} beforeUser={false} onOpenImage={() => undefined} />}
          {status.kind === 'failed' ? (
            <div role="alert" className="flex flex-col items-start gap-2 px-1">
              <p className="text-body text-danger">The thread did not start</p>
              <p className="max-w-full text-small break-words text-soft">{status.text}</p>
              <button className="btn h-7 px-3 text-label" onClick={() => report(() => api.retryThreadStart(thread.id))}>
                Retry
              </button>
            </div>
          ) : (
            <p role="status" className="flex h-6 items-center gap-2 px-1 text-body text-soft">
              <span className="flex flex-none animate-spin text-dim">
                <LoaderCircle size={14} aria-hidden="true" />
              </span>
              {status.text}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}
