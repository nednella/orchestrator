import { useEffect, useRef } from 'react'
import type { CSSProperties, ReactNode } from 'react'
import { useOrchestrator } from '../OrchestratorContext'

type Size = 'default' | 'reading' | 'wide' | 'settings'

const CENTRED: Record<Size, CSSProperties> = {
  default: { top: '12vh', width: 'min(40rem, 92vw)', maxHeight: '70vh', transform: 'translateX(-50%)' },
  reading: { top: '50%', width: 'min(44rem, 92vw)', maxHeight: '84vh', transform: 'translate(-50%, -50%)' },
  wide: { top: '2.5vh', width: 'min(72rem, 94vw)', maxHeight: '95vh', transform: 'translateX(-50%)' },
  settings: { top: '50%', width: 'min(80rem, calc(100vw - 3rem))', height: 'min(60rem, calc(100vh - 3rem))', transform: 'translate(-50%, -50%)' },
}

type OverlayProps = {
  label: string
  size?: Size
  onClose(): void
  children: ReactNode
}

export function Overlay({ label, size = 'default', onClose, children }: OverlayProps) {
  const { focus } = useOrchestrator('focus')
  const previous = useRef(document.activeElement)
  const dialog = useRef<HTMLDivElement>(null)

  useEffect(
    () => () => {
      // Strict Mode's rehearsal unmount leaves the dialog in the page.
      if (dialog.current?.isConnected) return
      const el = previous.current
      if (el instanceof HTMLElement && el.isConnected && el !== document.body) el.focus()
      else focus('thread')
    },
    [focus],
  )

  return (
    <div
      className="fade-in fixed inset-0"
      style={{ zIndex: 'var(--z-overlay)', background: 'var(--bg-overlay)' }}
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
    >
      <div
        ref={dialog}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        className={`absolute flex flex-col overflow-hidden border border-line-strong bg-surface shadow-2xl ${size === 'settings' ? 'rounded-2xl' : 'rounded-md'}`}
        style={{ ...CENTRED[size], left: '50%' }}
      >
        {children}
      </div>
    </div>
  )
}
