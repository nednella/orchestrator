import { useEffect, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos, useLatestAgentos } from '../AgentosContext'
import { api } from '../api'
import { escapeAction, lastTurnEnd } from '../chatComposer'
import type { EscapeAction } from '../chatComposer'
import type { Session } from '../types'
import { useChat } from '../useChat'
import { isTyping } from '../useListNav'
import { ChatNotice } from './ChatNotice'
import { ChatTimeline } from './ChatTimeline'
import { Loading } from './Loading'
import { Notice } from './Notice'
import { ThreadComposer } from './ThreadComposer'

// A stop that gets no turn end in this long can be asked for again.
const stopWait = 10_000

type ChatViewProps = { session: Session }

export function ChatView({ session }: ChatViewProps) {
  const { project, report } = useAgentos('project', 'report')
  const latest = useLatestAgentos()
  const { timeline, events, loading, error } = useChat(session.id)
  const empty = timeline.items.length === 0
  const lastEnd = lastTurnEnd(events)
  // The last turn end before the stop was asked for: the stop holds until a later one.
  const [stopAsked, setStopAsked] = useState<number | null>(null)
  const stopping = timeline.turnOpen && stopAsked === lastEnd

  useEffect(() => {
    if (!stopping) return
    const timer = setTimeout(() => setStopAsked(null), stopWait)
    return () => clearTimeout(timer)
  }, [stopping])

  const stopWith = (call: () => Promise<void>) => {
    if (stopping) return
    setStopAsked(lastEnd)
    report(() =>
      call().catch((err) => {
        setStopAsked(null)
        throw err
      }),
    )
  }
  const stop = () => stopWith(() => api.chatInterrupt(session.id))
  const sendNow = (message: string) => stopWith(() => api.chatSendNow(session.id, message))

  const escape = (picker: boolean): EscapeAction => {
    const action = escapeAction({ overlay: latest().overlay !== null, picker, turnOpen: timeline.turnOpen, stopping })
    if (action === 'interrupt') stop()
    return action
  }

  // A text field, the composer among them, takes its own Esc.
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== 'Escape' || isTyping(e.target)) return
    if (escape(false) === 'interrupt') e.preventDefault()
  }

  return (
    <div data-chat className="flex h-full min-h-0 flex-col bg-app" onKeyDown={onKeyDown}>
      {empty && loading && <Loading />}
      {empty && !loading && error && <Notice title="The chat did not load" hint={error} centered />}
      {empty && !loading && !error && <div className="flex-1" />}
      {!empty && <ChatTimeline id={session.id} items={timeline.items} folder={session.worktree || project?.dir || ''} />}
      <div className="mx-auto w-full max-w-[46rem] px-4 pb-4">
        {!empty && error && (
          <div className="pb-2">
            <ChatNotice text={error} error />
          </div>
        )}
        <ThreadComposer session={session} timeline={timeline} events={events} stopping={stopping} onStop={stop} onSendNow={sendNow} onEscape={escape} />
      </div>
    </div>
  )
}
