import { useCallback, useRef } from 'react'
import { useAgentosSelector } from '../AgentosContext'
import { CARD_ROOM } from '../contextCard'
import { useCardPlacement, useContextCard } from '../contextCardOpen'
import { useFocusRequest } from '../useFocusRequest'
import { ChatView } from './ChatView'
import { ThreadContextCard } from './ThreadContextCard'

const clearOfCard = { paddingRight: CARD_ROOM }

export function Viewport() {
  const selected = useAgentosSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const panel = useRef<HTMLElement>(null)
  const card = useContextCard()
  useCardPlacement(panel, selected !== undefined)

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
  const cardInline = card.shown && card.placement === 'inline'

  return (
    <section ref={panel} data-panel="thread" data-state={selected.state} tabIndex={-1} className="relative h-full min-h-0 min-w-0 overflow-hidden bg-app">
      {/* The chat keeps clear of the card, so the card never covers a message. */}
      <div className="h-full" style={cardInline ? clearOfCard : undefined}>
        <ChatView key={selected.id} session={selected} />
      </div>
      {cardInline && <ThreadContextCard session={selected} className="absolute top-3 right-3 z-20 max-h-[calc(100%-1.5rem)]" />}
    </section>
  )
}
