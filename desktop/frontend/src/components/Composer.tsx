import { useState } from 'react'
import type { DragEvent, ReactNode, RefObject } from 'react'
import type { ComposerDraft } from '../composerMarkdown'
import { ComposerEditor } from './ComposerEditor'
import type { ComposerInput } from './ComposerEditor'
import { Icon } from './Icon'

type ComposerProps = {
  inputRef: RefObject<ComposerInput | null>
  value: ComposerDraft
  label: string
  placeholder: string
  canSend: boolean
  sendLabel: string
  sendText?: string
  banner?: ReactNode
  drawer?: ReactNode
  attachments?: ReactNode
  left?: ReactNode
  right?: ReactNode
  // stop puts a Stop button in the send button's place.
  stop?: { stopping: boolean; onStop(): void }
  onChange(draft: ComposerDraft): void
  onCaret?(before: string, after: string): void
  onKeyDown?(e: KeyboardEvent): void
  onAttach?(data: DataTransfer): boolean
  onSend(): void
}

export function Composer({ inputRef, value, label, placeholder, canSend, sendLabel, sendText, banner, drawer, attachments, left, right, stop, onChange, onCaret, onKeyDown, onAttach, onSend }: ComposerProps) {
  const [dragging, setDragging] = useState(false)

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
    <div className="mx-auto w-full max-w-[46rem]">
      {(banner || drawer) && (
        <div className="composer-attached fade-in">
          {banner}
          {drawer}
        </div>
      )}
      <div
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
          <div className="flex min-w-0 flex-1 items-center gap-0.5">{left}</div>
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
              <Icon name="stop" size={14} />
            </button>
          ) : (
            <button
              className="composer-send"
              data-text={Boolean(sendText)}
              disabled={!canSend}
              aria-label={sendLabel}
              title={sendLabel}
              onMouseDown={(e) => e.preventDefault()}
              onClick={onSend}
            >
              {sendText ?? <Icon name="arrow-up" size={14} />}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
