import { useCallback, useRef } from 'react'
import { useAgentosSelector } from '../AgentosContext'
import { useFocusRequest } from '../useFocusRequest'
import { ChatView } from './ChatView'

export function Viewport() {
  const selected = useAgentosSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const panel = useRef<HTMLElement>(null)

  useFocusRequest('thread', panel, !selected)
  // The thread's chat takes the focus in its scroll area, or in the panel while the chat loads.
  const focusChat = useCallback(() => {
    const section = panel.current
    if (!section?.querySelector('[data-chat]')) return
    const target = section.querySelector<HTMLElement>('[data-chat-scroll]') ?? section
    target.focus()
  }, [])
  useFocusRequest('thread', focusChat)

  if (!selected) return null

  return (
    <section ref={panel} data-panel="thread" data-state={selected.state} tabIndex={-1} className="relative h-full min-h-0 min-w-0 overflow-hidden bg-app">
      <ChatView key={selected.id} session={selected} />
    </section>
  )
}
