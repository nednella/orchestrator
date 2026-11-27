import { SquareArrowOutUpRight, X } from 'lucide-react'
import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'
import type { BrowserState } from '../types'
import { Ago } from './Ago'
import { CaptureButton } from './CaptureButton'
import { ChatActionButton } from './ChatActionButton'
import { Spinner } from './Spinner'

type BrowserStatusHeaderProps = { id: string; state: BrowserState }

export function BrowserStatusHeader({ id, state }: BrowserStatusHeaderProps) {
  const { report } = useOrchestrator('report')

  return (
    <header className="flex flex-none flex-wrap items-center gap-x-3 gap-y-2 border-b border-(--border-rule) px-4 py-3">
      <div className="min-w-0 flex-1 basis-48">
        <h3 className="truncate text-body font-medium" title={state.title}>
          {state.title || 'Untitled page'}
        </h3>
        <p className="mt-0.5 flex items-center gap-2 text-small text-dim">
          <span className="mono min-w-0 truncate" title={state.url}>
            {state.url.replace(/^https?:\/\//, '')}
          </span>
          {state.loading ? (
            <span className="flex flex-none items-center gap-1.5">
              <Spinner size={12} muted />
              loading
            </span>
          ) : (
            <span className="flex-none">{state.loadedAt ? <>· loaded <Ago at={state.loadedAt} /></> : ''}</span>
          )}
          {state.pages > 1 && <span className="flex-none">· {state.pages} tabs</span>}
        </p>
      </div>
      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        <ChatActionButton label="Bring the browser window to the front" onClick={() => report(() => api.browserShow(id))}>
          <SquareArrowOutUpRight size={14} aria-hidden="true" />
          Show window
        </ChatActionButton>
        <CaptureButton id={id} />
        <ChatActionButton label="Close this browser window" onClick={() => report(() => api.browserClose(id))}>
          <X size={14} aria-hidden="true" />
        </ChatActionButton>
      </div>
    </header>
  )
}
