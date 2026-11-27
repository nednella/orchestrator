import type { BrowserState, Thread } from '../types'
import { BrowserConsoleList } from './BrowserConsoleList'
import { BrowserRecentCaptures } from './BrowserRecentCaptures'
import { BrowserStatusHeader } from './BrowserStatusHeader'

type BrowserStatusProps = { thread: Thread; state: BrowserState }

export function BrowserStatus({ thread, state }: BrowserStatusProps) {
  return (
    <div className="absolute inset-0 flex flex-col">
      <BrowserStatusHeader id={thread.id} state={state} />
      <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto p-4">
        <BrowserConsoleList lines={state.console} />
        <BrowserRecentCaptures thread={thread} />
      </div>
    </div>
  )
}
