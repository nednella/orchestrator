import { useCallback, useRef } from 'react'
import { useAgentosSelector } from '../AgentosContext'
import { useFocusRequest } from '../useFocusRequest'
import { ChatView } from './ChatView'
import { Terminal } from './Terminal'

export function Viewport() {
  const selected = useAgentosSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const panel = useRef<HTMLElement>(null)

  useFocusRequest('thread', panel, !selected)
  // A hosted thread's chat takes the focus in its scroll area, or in the panel while the chat loads.
  const focusChat = useCallback(() => {
    const section = panel.current
    if (!section?.querySelector('[data-chat]')) return
    const target = section.querySelector<HTMLElement>('[data-chat-scroll]') ?? section
    target.focus()
  }, [])
  useFocusRequest('thread', focusChat)

  if (!selected) return null

  return (
    <section ref={panel} data-panel="thread" data-state={selected.state} tabIndex={-1} className={`relative h-full min-h-0 min-w-0 overflow-hidden ${selected.hosted ? 'bg-app' : 'bg-term'}`}>
      {selected.hosted ? <ChatView key={selected.id} session={selected} /> : <Terminal key={selected.id} id={selected.id} active />}
    </section>
  )
}
