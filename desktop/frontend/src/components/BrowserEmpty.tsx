import { CircleAlert, Globe } from 'lucide-react'
import { useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import type { Thread } from '../types'

type BrowserEmptyProps = { thread: Thread; error: string }

export function BrowserEmpty({ thread, error }: BrowserEmptyProps) {
  const { openBrowser, report } = useOrchestrator('openBrowser', 'report')
  const [url, setUrl] = useState('')

  return (
    <div className="absolute inset-0 grid place-items-center overflow-y-auto p-6">
      <form
        className="flex w-full max-w-md flex-col gap-3 text-center"
        onSubmit={(e) => {
          e.preventDefault()
          report(() => openBrowser(thread.id, url.trim()))
        }}
      >
        <Globe size={20} className="mx-auto text-dim" aria-hidden="true" />
        <div className="flex flex-col gap-1">
          <h3 className="text-body font-medium">No page open</h3>
          <p className="text-small text-dim">The agent can drive this browser, and so can you. It stays logged in across this project.</p>
        </div>
        <div className="flex gap-2">
          <input
            value={url}
            autoFocus
            spellCheck={false}
            aria-label="Address"
            placeholder="Address, or leave empty for the project page"
            className="setting-field h-8 min-w-0 flex-1 px-2.5 text-ink placeholder:text-dim"
            onChange={(e) => setUrl(e.target.value)}
          />
          <button type="submit" className="btn btn-primary h-8 rounded-full">
            Open
          </button>
        </div>
        {error && (
          <p className="flex items-center justify-center gap-1.5 text-small text-danger">
            <CircleAlert size={12} className="flex-none" aria-hidden="true" />
            {error}
          </p>
        )}
      </form>
    </div>
  )
}
