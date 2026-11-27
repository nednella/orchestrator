import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'

// ChatStartFailed is where the working row was when a new thread's start failed: why, and a retry that goes on from
// the step that failed.
type ChatStartFailedProps = { thread: string; text: string }

export function ChatStartFailed({ thread, text }: ChatStartFailedProps) {
  const { report } = useOrchestrator('report')
  return (
    <div role="alert" className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 px-1 py-1 text-small">
      <span className="min-w-0 break-words text-danger">The thread did not start: {text}</span>
      <button className="btn h-6 flex-none px-2 text-label" onClick={() => report(() => api.retryThreadStart(thread))}>
        Retry
      </button>
    </div>
  )
}
