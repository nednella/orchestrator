import { useState } from 'react'
import type { ChangeEvent, DragEvent, KeyboardEvent, ReactNode, RefObject, SyntheticEvent } from 'react'
import { ComposerEditor } from './ComposerEditor'
import { Icon } from './Icon'

type ComposerProps = {
  taRef: RefObject<HTMLTextAreaElement>
  value: string
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
  onChange(e: ChangeEvent<HTMLTextAreaElement>): void
  onSelect?(e: SyntheticEvent<HTMLTextAreaElement>): void
  onKeyDown?(e: KeyboardEvent<HTMLTextAreaElement>): void
  onAttach?(data: DataTransfer): boolean
  onSend(): void
}

export function Composer({ taRef, value, label, placeholder, canSend, sendLabel, sendText, banner, drawer, attachments, left, right, onChange, onSelect, onKeyDown, onAttach, onSend }: ComposerProps) {
  const [dragging, setDragging] = useState(false)

  const keyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    onKeyDown?.(e)
    if (e.defaultPrevented || e.key !== 'Enter' || e.shiftKey || e.nativeEvent.isComposing) return
    e.preventDefault()
    onSend()
  }

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
            taRef={taRef}
            value={value}
            label={label}
            placeholder={placeholder}
            onChange={onChange}
            onSelect={onSelect}
            onKeyDown={keyDown}
            onPaste={(e) => onAttach?.(e.clipboardData) && e.preventDefault()}
          />
        </div>
        <div className="flex items-center gap-2 px-2 pb-2">
          <div className="flex min-w-0 flex-1 items-center gap-0.5">{left}</div>
          <div className="flex min-w-0 items-center justify-end gap-0.5">{right}</div>
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
        </div>
      </div>
    </div>
  )
}
