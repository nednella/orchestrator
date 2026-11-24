import { useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useAgentos } from '../AgentosContext'
import { EFFORTS, modelName } from '../model'
import type { Session } from '../types'
import { ThreadCardLine } from './ThreadCardLine'

// anchor is the row's box: the card sits beside the sidebar, or below or above the row when the window is too narrow.
type ThreadCardProps = { id: string; session: Session; anchor: DOMRect }

const GAP = 12
const MARGIN = 8

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(value, max))

export function ThreadCard({ id, session, anchor }: ThreadCardProps) {
  const { machine } = useAgentos('machine')
  const card = useRef<HTMLDivElement>(null)
  const [place, setPlace] = useState<{ left: number; top: number } | null>(null)
  const model = modelName(session.model)
  const effort = EFFORTS[session.effort] ?? session.effort
  const claude = session.agent === 'claude'

  useLayoutEffect(() => {
    const el = card.current
    if (!el) return
    const { width, height } = el.getBoundingClientRect()
    const beside = anchor.right + GAP
    if (beside + width <= window.innerWidth - MARGIN) {
      setPlace({ left: beside, top: clamp(anchor.top, MARGIN, window.innerHeight - height - MARGIN) })
      return
    }
    const below = anchor.bottom + 4
    const top = below + height <= window.innerHeight - MARGIN ? below : anchor.top - 4 - height
    setPlace({ left: clamp(anchor.left, MARGIN, window.innerWidth - width - MARGIN), top })
  }, [anchor])

  return createPortal(
    <div
      ref={card}
      id={id}
      role="tooltip"
      className="popup pointer-events-none flex w-[17.5rem] max-w-[calc(100vw-1rem)] flex-col gap-2 px-3 py-2.5"
      style={place ?? { left: anchor.right, top: anchor.top, opacity: 0 }}
    >
      <p className="text-body font-semibold break-words text-ink">{session.title}</p>
      <div className="flex flex-col gap-1.5 text-small text-soft">
        {machine && <ThreadCardLine icon="machine">{machine}</ThreadCardLine>}
        {(model || effort) && (
          <ThreadCardLine icon={claude ? 'claude' : 'agent'} iconColor={claude ? 'var(--agent-claude)' : undefined}>
            <span className="flex gap-1.5">
              {model && <span className="text-ink">{model}</span>}
              {effort && <span>{effort}</span>}
            </span>
          </ThreadCardLine>
        )}
      </div>
    </div>,
    document.body,
  )
}
