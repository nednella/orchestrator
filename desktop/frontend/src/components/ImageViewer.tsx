import { X } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { ChatActionButton } from './ChatActionButton'
import { ViewerBar } from './ViewerBar'

type ImageViewerProps = { url: string; alt?: string; onClose(): void }

export function ImageViewer({ url, alt = 'Note attachment', onClose }: ImageViewerProps) {
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => root.current?.focus(), [])

  return (
    <div
      ref={root}
      tabIndex={-1}
      className="fade-in fixed inset-0 grid place-items-center p-6 outline-none"
      style={{ zIndex: 'var(--z-overlay)', background: 'var(--bg-overlay-strong)' }}
      role="dialog"
      aria-label="Image"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
      onKeyDown={(e) => {
        if (e.key !== 'Escape') return
        // Esc in a thread would also stop its turn.
        e.stopPropagation()
        onClose()
      }}
    >
      <div className="relative max-h-full max-w-full">
        <img src={url} alt={alt} className="block max-h-[85vh] max-w-full rounded-lg border border-(--border-rule) object-contain" />
        <div className="absolute top-2 right-2">
          <ViewerBar>
            <ChatActionButton label="Close image" onClick={onClose}>
              <X size={14} aria-hidden="true" />
            </ChatActionButton>
          </ViewerBar>
        </div>
      </div>
    </div>
  )
}
