import { useRef } from 'react'
import { useAgentosSelector } from '../AgentosContext'
import { useFocusRequest } from '../useFocusRequest'
import { Terminal } from './Terminal'

export function Viewport() {
  const selected = useAgentosSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const panel = useRef<HTMLElement>(null)

  useFocusRequest('terminal', panel, !selected)

  if (!selected) return null

  return (
    <section ref={panel} data-panel="terminal" data-state={selected.state} tabIndex={-1} className="relative h-full min-h-0 min-w-0 overflow-hidden bg-term">
      <Terminal key={selected.id} id={selected.id} active />
    </section>
  )
}
