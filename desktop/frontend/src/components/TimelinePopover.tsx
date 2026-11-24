import { useRef } from 'react'
import { createPortal } from 'react-dom'
import type { Session } from '../types'
import { useFloatingAt } from '../useFloatingAt'
import type { Point } from '../useFloatingAt'
import { Timeline } from './Timeline'

type TimelinePopoverProps = { session: Session; at: Point; onClose(): void }

export function TimelinePopover({ session, at, onClose }: TimelinePopoverProps) {
  const popup = useRef<HTMLDivElement>(null)
  const style = useFloatingAt(popup, at, onClose)

  return createPortal(
    <div ref={popup} role="dialog" aria-label={`Timeline of ${session.title}`} className="popup fade-in p-3" style={style}>
      <Timeline session={session} size="full" />
    </div>,
    document.body,
  )
}
