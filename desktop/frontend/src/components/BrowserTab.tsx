import { useEffect } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import type { Thread } from '../types'
import { BrowserEmpty } from './BrowserEmpty'
import { BrowserStatus } from './BrowserStatus'

type BrowserTabProps = { thread: Thread }

export function BrowserTab({ thread }: BrowserTabProps) {
  const { browserStates, loadBrowserState } = useOrchestrator('browserStates', 'loadBrowserState')
  const state = browserStates[thread.id]
  const known = Boolean(state)

  useEffect(() => {
    if (thread.browser && !known) void loadBrowserState(thread.id)
  }, [thread.id, thread.browser, known, loadBrowserState])

  if (!state?.open) return <BrowserEmpty thread={thread} error={state?.error ?? ''} />
  return <BrowserStatus thread={thread} state={state} />
}
