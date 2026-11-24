import { useEffect, useRef, useState } from 'react'
import { useClock } from '../time'
import type { Session } from '../types'
import { Timeline, timelineSummary } from './Timeline'

type TimelineButtonProps = { session: Session }

export function TimelineButton({ session }: TimelineButtonProps) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const summary = useClock((now) => timelineSummary(session, now))

  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    window.addEventListener('mousedown', close)
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('mousedown', close)
      window.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div ref={root} className="min-w-0">
      <button className="mono rounded-sm text-label whitespace-nowrap text-dim hover:text-soft" aria-expanded={open} title="Show the timeline" onClick={() => setOpen(!open)}>
        {summary || 'just started'}
      </button>
      {open && (
        <div className="fade-in absolute top-full left-4 mt-1 rounded-md border border-line-strong bg-raised p-3" style={{ zIndex: 'var(--z-popup)' }}>
          <Timeline session={session} size="full" />
        </div>
      )}
    </div>
  )
}
