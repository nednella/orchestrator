import { useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useAgentos } from '../AgentosContext'
import { EFFORTS, modelName } from '../model'
import { cleanupNote } from '../threads'
import type { Session } from '../types'
import { PILL } from './PRCard'
import { ThreadCardLine } from './ThreadCardLine'

// anchor is the row's box: the card sits beside the sidebar, or below or above the row when the window is too narrow.
type ThreadCardProps = { id: string; session: Session; anchor: DOMRect }

const GAP = 12
const MARGIN = 8

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(value, max))

export function ThreadCard({ id, session, anchor }: ThreadCardProps) {
  const { projects, machine } = useAgentos('projects', 'machine')
  const card = useRef<HTMLDivElement>(null)
  const [place, setPlace] = useState<{ left: number; top: number } | null>(null)
  const project = projects.find((p) => p.key === session.project)?.name ?? session.project
  const model = [modelName(session.model), EFFORTS[session.effort] ?? session.effort].filter(Boolean).join(' · ')
  const note = cleanupNote(session)
  const { pr } = session

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
      <p className="text-small font-medium break-words text-ink">{session.title}</p>
      <div className="flex flex-col gap-1.5 text-label text-soft">
        <ThreadCardLine icon="folder">{project}</ThreadCardLine>
        {machine && <ThreadCardLine icon="machine">{machine}</ThreadCardLine>}
        {session.branch && (
          <ThreadCardLine icon="git-branch">
            <span className="mono">{session.branch}</span>
          </ThreadCardLine>
        )}
        {model && <ThreadCardLine icon="model">{model}</ThreadCardLine>}
        {pr && (
          <ThreadCardLine icon={PILL[pr.state].icon} iconColor={`var(--pr-${pr.state})`}>
            PR #{pr.number} · {PILL[pr.state].label}
          </ThreadCardLine>
        )}
        {note && (
          <ThreadCardLine icon={session.cleanup === 'blocked' ? 'alert' : 'broom'} danger={session.cleanup === 'blocked'}>
            {note}
          </ThreadCardLine>
        )}
      </div>
    </div>,
    document.body,
  )
}
