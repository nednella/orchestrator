import { Square, ArrowUp, LoaderCircle } from 'lucide-react'
import { useLayoutEffect, useRef, useState } from 'react'
import type { DragEvent, ReactNode, RefObject } from 'react'
import { footerMode } from '../composerLayout'
import type { FooterMode } from '../composerLayout'
import type { ComposerDraft } from '../composerMarkdown'
import { ComposerEditor } from './ComposerEditor'
import type { ComposerInput } from './ComposerEditor'

// left may lay the footer's controls out for its mode, which the width of the form picks.
type ComposerProps = {
  inputRef: RefObject<ComposerInput | null>
  value: ComposerDraft
  label: string
  placeholder?: string
  canSend: boolean
  sendLabel: string
  sendText?: string
  sending?: boolean
  banner?: ReactNode
  drawer?: ReactNode
  attachments?: ReactNode
  left?: ReactNode | ((footer: FooterMode) => ReactNode)
  right?: ReactNode
  stop?: { stopping: boolean; onStop(): void }
  onChange(draft: ComposerDraft): void
  onCaret?(before: string, after: string): void
  onKeyDown?(e: KeyboardEvent): void
  onAttach?(data: DataTransfer): boolean
  onSend(): void
}

export function Composer({ inputRef, value, label, placeholder, canSend, sendLabel, sendText, sending = false, banner, drawer, attachments, left, right, stop, onChange, onCaret, onKeyDown, onAttach, onSend }: ComposerProps) {
  const [dragging, setDragging] = useState(false)
  const box = useRef<HTMLDivElement>(null)
  const [footer, setFooter] = useState<FooterMode>('wide')

  useLayoutEffect(() => {
    const form = box.current
    if (!form) return
    setFooter(footerMode(form.offsetWidth))
    const observer = new ResizeObserver(() => setFooter(footerMode(form.offsetWidth)))
    observer.observe(form)
    return () => observer.disconnect()
  }, [])

  const dragOver = (e: DragEvent) => {
    if (!onAttach || !e.dataTransfer.types.includes('Files')) return
    e.preventDefault()
    setDragging(true)
  }

  const drop = (e: DragEvent) => {
    setDragging(false)
    if (onAttach?.(e.dataTransfer)) e.preventDefault()
  }

  return (
    <div className="chat-column">
      {(banner || drawer) && (
        <div className="composer-attached fade-in">
          {banner}
          {drawer}
        </div>
      )}
      <div
        ref={box}
        className="composer-box"
        data-dragging={dragging}
        onDragOver={dragOver}
        onDragLeave={(e) => !e.currentTarget.contains(e.relatedTarget as Node) && setDragging(false)}
        onDrop={drop}
      >
        <div className="px-4 pt-3.5 pb-2 sm:pt-4">
          {attachments && <div className="mb-3 flex gap-2">{attachments}</div>}
          <ComposerEditor
            inputRef={inputRef}
            value={value}
            label={label}
            placeholder={placeholder}
            onChange={onChange}
            onCaret={onCaret}
            onKeyDown={onKeyDown}
            onAttach={onAttach}
            onSend={onSend}
          />
        </div>
        <div className="flex items-center gap-2 px-2 pb-2">
          <div className="flex flex-1 items-center gap-0.5">{typeof left === 'function' ? left(footer) : left}</div>
          <div className="flex min-w-0 items-center justify-end gap-0.5">{right}</div>
          {stop ? (
            <button
              className="composer-send"
              data-stop="true"
              disabled={stop.stopping}
              aria-label={stop.stopping ? 'Stopping' : 'Stop'}
              title={stop.stopping ? 'Stopping' : 'Stop (Esc)'}
              onMouseDown={(e) => e.preventDefault()}
              onClick={stop.onStop}
            >
              <Square size={10} fill="currentColor" aria-hidden="true" />
            </button>
          ) : (
            <button
              className="composer-send"
              data-text={Boolean(sendText)}
              disabled={!canSend || sending}
              aria-label={sending ? 'Sending' : sendLabel}
              title={sending ? 'Sending' : sendLabel}
              onMouseDown={(e) => e.preventDefault()}
              onClick={onSend}
            >
              {sending ? (
                <span className="flex animate-spin">
                  <LoaderCircle size={14} aria-hidden="true" />
                </span>
              ) : (
                (sendText ?? <ArrowUp size={14} aria-hidden="true" />)
              )}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
