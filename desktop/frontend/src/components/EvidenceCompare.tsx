import { X } from 'lucide-react'
import { useEffect, useRef } from 'react'
import type { Evidence } from '../types'
import { ChatActionButton } from './ChatActionButton'
import { ViewerBar } from './ViewerBar'

type EvidenceCompareProps = { left: Evidence; right: Evidence; onClose(): void }

export function EvidenceCompare({ left, right, onClose }: EvidenceCompareProps) {
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => root.current?.focus(), [])

  return (
    <div
      ref={root}
      tabIndex={-1}
      role="dialog"
      aria-label="Compare media"
      className="fade-in fixed inset-0 flex flex-col items-center justify-center gap-3 p-4 outline-none"
      style={{ zIndex: 'var(--z-overlay)', background: 'var(--bg-overlay-strong)' }}
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
      onKeyDown={(e) => e.key === 'Escape' && onClose()}
    >
      <div className="grid w-full max-w-6xl grid-cols-1 gap-4 overflow-y-auto md:grid-cols-2">
        {[left, right].map((item, i) => (
          <figure key={item.id} className="flex min-w-0 flex-col gap-2">
            <figcaption className="text-small font-medium text-soft">{i === 0 ? 'Left' : 'Right'}</figcaption>
            <img src={item.url} alt={item.caption || 'Image'} className="block max-h-[62vh] w-full rounded-lg border border-(--border-rule) object-contain" />
            <p className="text-small text-soft">{item.caption || 'No caption'}</p>
          </figure>
        ))}
      </div>
      <ViewerBar>
        <ChatActionButton label="Close" onClick={onClose}>
          <X size={14} aria-hidden="true" />
          Close
        </ChatActionButton>
      </ViewerBar>
    </div>
  )
}
