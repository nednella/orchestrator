import { Bot } from 'lucide-react'
import { useState } from 'react'
import type { CSSProperties } from 'react'
import { useOrchestrator, useOrchestratorSelector } from '../OrchestratorContext'
import { STATE_LABEL } from '../stateMeta'
import { needsOwner } from '../threads'
import type { Session } from '../types'
import type { Point } from '../useFloatingAt'
import { ChildMenu } from './ChildMenu'
import { LastActive } from './LastActive'
import { StateDot } from './StateDot'

// depth is how far under the open thread the child is, 0 for one it delegated to itself.
type AgentThreadRowProps = { child: Session; depth: number }

// AgentThreadRow is a delegated child in the Agents view: a click opens it in the main view, a right-click gives its
// menu. A child that waits, or has a child of its own that waits, shows as waiting.
export function AgentThreadRow({ child, depth }: AgentThreadRowProps) {
  const a = useOrchestrator('select', 'focus')
  const waits = useOrchestratorSelector((s) => needsOwner(child, s.sessions) !== undefined)
  const [menuAt, setMenuAt] = useState<Point | null>(null)
  const state = waits ? 'waiting' : child.state
  const indent = { '--depth': depth } as CSSProperties

  return (
    <>
      <button
        className="flex h-8 w-full min-w-0 flex-none items-center gap-2.5 rounded-lg pr-2.5 pl-[calc(0.625rem+var(--depth)*1rem)] text-left text-body hover:bg-(--bg-row-hover)"
        style={indent}
        onClick={() => {
          a.select(child.id)
          a.focus('thread')
        }}
        onContextMenu={(e) => {
          e.preventDefault()
          setMenuAt({ x: e.clientX, y: e.clientY })
        }}
      >
        <span className="relative flex flex-none text-soft">
          <Bot size={16} aria-hidden="true" />
          <span className="absolute -right-1 bottom-0 flex rounded-full bg-app p-[2px]">
            <StateDot state={state} />
          </span>
        </span>
        <span className="min-w-0 truncate">{child.title}</span>
        <span className="ml-auto flex flex-none items-center gap-2 text-small text-dim tabular-nums">
          <span className={waits ? 'text-ink' : undefined}>{STATE_LABEL[state]}</span>
          <LastActive at={child.lastEventAt} />
        </span>
      </button>
      {menuAt && <ChildMenu child={child} at={menuAt} onClose={() => setMenuAt(null)} />}
    </>
  )
}
