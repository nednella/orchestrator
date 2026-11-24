import { useRef } from 'react'
import { useAgentosSelector } from '../AgentosContext'
import { useFocusRequest } from '../useFocusRequest'
import { Terminal } from './Terminal'
import { ViewportHeader } from './ViewportHeader'

export function Viewport() {
  const selected = useAgentosSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const panel = useRef<HTMLElement>(null)

  useFocusRequest('terminal', panel, !selected)

  if (!selected) return null

  return (
    <section
      ref={panel}
      data-panel="terminal"
      data-state={selected.state}
      tabIndex={-1}
      className="panel flex h-full min-h-0 min-w-0 flex-col overflow-hidden"
    >
      <ViewportHeader key={selected.id} session={selected} />
      <div className="relative min-h-0 flex-1 border-t border-line bg-term">
        <Terminal key={selected.id} id={selected.id} active />
      </div>
    </section>
  )
}
