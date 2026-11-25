import type { Session } from '../types'
import { useChat } from '../useChat'
import { ChatNotice } from './ChatNotice'
import { ChatTimeline } from './ChatTimeline'
import { Loading } from './Loading'
import { Notice } from './Notice'

type ChatViewProps = { session: Session }

export function ChatView({ session }: ChatViewProps) {
  const { timeline, loading, error } = useChat(session.id)
  const empty = timeline.items.length === 0

  return (
    <div data-chat className="flex h-full min-h-0 flex-col bg-app">
      {empty && loading && <Loading />}
      {empty && !loading && error && <Notice title="The chat did not load" hint={error} centered />}
      {empty && !loading && !error && <div className="flex-1" />}
      {!empty && <ChatTimeline id={session.id} items={timeline.items} />}
      <div className="mx-auto w-full max-w-[46rem] px-4 pb-4">
        {!empty && error && (
          <div className="pb-2">
            <ChatNotice text={error} error />
          </div>
        )}
        <div className="composer-box px-4 py-3.5 text-body text-dim">Sending from the chat is not built yet</div>
      </div>
    </div>
  )
}
