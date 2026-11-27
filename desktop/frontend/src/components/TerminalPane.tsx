import { useEffect, useRef } from 'react'
import { useLayout } from '../LayoutContext'
import { shellTerminal } from '../terminal/shellTerminal'

type TerminalPaneProps = { id: string; divider: 'none' | 'active' | 'idle'; edge: 'left' | 'top'; onActivate(): void }

const DIVIDER = { none: undefined, active: '1px solid var(--border)', idle: '1px solid var(--border-terminal-soft)' }

// TerminalPane shows the shell's terminal; the terminal stays alive when the pane goes, so it comes back as it was.
// edge is the side of the pane that its divider is on.
export function TerminalPane({ id, divider, edge, onActivate }: TerminalPaneProps) {
  const host = useRef<HTMLDivElement>(null)
  const { focusChat } = useLayout()

  useEffect(() => {
    const terminal = shellTerminal(id)
    terminal.mount(host.current!)
    return () => terminal.unmount()
  }, [id])

  useEffect(() => {
    shellTerminal(id).onLeave = focusChat
  }, [id, focusChat])

  return (
    <div className="relative min-h-0 min-w-0" style={edge === 'left' ? { borderLeft: DIVIDER[divider] } : { borderTop: DIVIDER[divider] }} onPointerDown={onActivate}>
      <div ref={host} className="absolute inset-0" />
    </div>
  )
}
