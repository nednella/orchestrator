import { useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useAgentos } from '../AgentosContext'
import { EFFORTS, modelName } from '../model'
import { threadCardLines } from '../threadCard'
import type { Session } from '../types'
import { ThreadCardLine } from './ThreadCardLine'

// anchor is the row's box: the card sits beside the sidebar, or below or above the row when the window is too narrow.
type ThreadCardProps = { id: string; session: Session; anchor: DOMRect }

const GAP = 12
const MARGIN = 8

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(value, max))

export function ThreadCard({ id, session, anchor }: ThreadCardProps) {
  const { machine, issues } = useAgentos('machine', 'issues')
  const card = useRef<HTMLDivElement>(null)
  const [place, setPlace] = useState<{ left: number; top: number } | null>(null)
  const lines = threadCardLines(session, machine, issues.find((i) => i.number === session.issue)?.title ?? '')

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
        {lines.map((line) => (
          <ThreadCardLine key={line.key} icon={line.icon} iconColor={line.color}>
            {line.key === 'model' ? <ModelText model={line.text} effort={line.detail ?? ''} /> : <LineText text={line.text} detail={line.detail} />}
          </ThreadCardLine>
        ))}
      </div>
    </div>,
    document.body,
  )
}

type ModelTextProps = { model: string; effort: string }

function ModelText({ model, effort }: ModelTextProps) {
  const name = modelName(model)
  const level = EFFORTS[effort] ?? effort
  return (
    <span className="flex gap-1.5">
      {name && <span className="text-ink">{name}</span>}
      {level && <span>{level}</span>}
    </span>
  )
}

type LineTextProps = { text: string; detail?: string }

function LineText({ text, detail }: LineTextProps) {
  return (
    <>
      {text}
      {detail && <span className="text-dim"> {'· ' + detail}</span>}
    </>
  )
}
