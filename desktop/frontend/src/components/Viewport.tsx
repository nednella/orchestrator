import { useCallback, useRef } from 'react'
import type { CSSProperties } from 'react'
import { useOrchestratorSelector } from '../OrchestratorContext'
import { CARD_ROOM } from '../contextCard'
import { useCardPlacement, useContextCard } from '../contextCardOpen'
import { viewedAgent } from '../navHistory'
import { useFocusRequest } from '../useFocusRequest'
import { ChatStarting } from './ChatStarting'
import { ChatView } from './ChatView'
import { SubagentView } from './SubagentView'
import { ThreadContextCard } from './ThreadContextCard'

const clearOfCard = { '--card-room': `${CARD_ROOM}px` } as CSSProperties

export function Viewport() {
  const selected = useOrchestratorSelector((a) => a.threads.find((s) => s.id === a.selectedId))
  const panel = useRef<HTMLElement>(null)
  const card = useContextCard()
  useCardPlacement(panel, selected !== undefined)
  const id = selected?.id ?? ''
  const agent = useOrchestratorSelector((a) => viewedAgent(a.viewHistory, id))

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
      {/* The chat's scroller spans the whole width, so its scroll bar stays at the edge; its rows keep clear of the card. */}
      <div className="h-full" style={cardInline ? clearOfCard : undefined}>
        {selected.start !== '' ? <ChatStarting key={selected.id} thread={selected} /> : agent ? <SubagentView key={`${selected.id}/${agent}`} parent={selected} agent={agent} /> : <ChatView key={selected.id} thread={selected} />}
      </div>
      {cardInline && <ThreadContextCard id={selected.id} className="absolute top-3 right-3 z-20 max-h-[calc(100%-1.5rem)]" />}
    </section>
  )
}
