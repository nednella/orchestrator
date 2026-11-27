import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'

// ChatStartFailed is where the working row was when a new thread's start failed or the owner cancelled it: why, and a
// retry that goes on from the step that stopped, with the messages the start held.
type ChatStartFailedProps = { thread: string; text: string; cancelled: boolean }

export function ChatStartFailed({ thread, text, cancelled }: ChatStartFailedProps) {
  const { report } = useOrchestrator('report')
  return (
    <div role={cancelled ? 'status' : 'alert'} className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 px-1 py-1 text-small">
      <span className={`min-w-0 break-words ${cancelled ? 'text-soft' : 'text-danger'}`}>{cancelled ? text : `The thread did not start: ${text}`}</span>
      <button className="btn h-6 flex-none px-2 text-label" onClick={() => report(() => api.retryThreadStart(thread))}>
        Retry
      </button>
    </div>
  )
}
