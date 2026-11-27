import { X } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { previewDialogFrameStyle, previewFrame, previewWidth } from '../questionPreview'
import { ChatActionButton } from './ChatActionButton'
import { ViewerBar } from './ViewerBar'

type QuestionHtmlDialogProps = { html: string; tokens: Record<string, string>; onClose(): void }

// QuestionHtmlDialog is an html block of a preview at its full size, in the same sandbox as in its tile.
export function QuestionHtmlDialog({ html, tokens, onClose }: QuestionHtmlDialogProps) {
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => root.current?.focus(), [])

  return (
    <div
      ref={root}
      tabIndex={-1}
      className="fade-in fixed inset-0 grid place-items-center p-6 outline-none"
      style={{ zIndex: 'var(--z-overlay)', background: 'var(--bg-overlay-strong)' }}
      role="dialog"
      aria-label="Preview"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
      onKeyDown={(e) => {
        if (e.key !== 'Escape') return
        e.stopPropagation()
        onClose()
      }}
    >
      <div className="relative h-full max-h-[85vh] w-full" style={{ maxWidth: previewWidth }}>
        <iframe title="Preview" {...previewFrame(html, tokens)} className="block h-full w-full rounded-lg border border-(--border-rule)" style={previewDialogFrameStyle} />
        <div className="absolute top-2 right-2">
          <ViewerBar>
            <ChatActionButton label="Close preview" onClick={onClose}>
              <X size={14} aria-hidden="true" />
            </ChatActionButton>
          </ViewerBar>
        </div>
      </div>
    </div>
  )
}
