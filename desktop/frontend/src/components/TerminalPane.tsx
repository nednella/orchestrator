import { useEffect, useRef } from 'react'
import { useLayout } from '../LayoutContext'
import { termSession } from '../terminal/session'

type TerminalPaneProps = { id: string; divider: 'none' | 'active' | 'idle'; onActivate(): void }

const DIVIDER = { none: undefined, active: '1px solid var(--border)', idle: '1px solid var(--border-terminal-soft)' }

// TerminalPane shows the shell's terminal; the terminal stays alive when the pane goes, so it comes back as it was.
export function TerminalPane({ id, divider, onActivate }: TerminalPaneProps) {
  const host = useRef<HTMLDivElement>(null)
  const { focusChat } = useLayout()

  useEffect(() => {
    const session = termSession(id)
    session.mount(host.current!)
    return () => session.unmount()
  }, [id])

  useEffect(() => {
    termSession(id).onLeave = focusChat
  }, [id, focusChat])

  return (
    <div className="relative min-h-0 min-w-0" style={{ borderLeft: DIVIDER[divider] }} onPointerDown={onActivate}>
      <div ref={host} className="absolute inset-0" />
    </div>
  )
}
