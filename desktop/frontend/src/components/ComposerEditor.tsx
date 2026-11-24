import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { ChangeEvent, ClipboardEvent, KeyboardEvent, RefObject, SyntheticEvent } from 'react'
import { markdownSpans } from '../markdown'
import { AutoTextarea } from './AutoTextarea'

type ComposerEditorProps = {
  taRef: RefObject<HTMLTextAreaElement>
  value: string
  label: string
  placeholder: string
  onChange(e: ChangeEvent<HTMLTextAreaElement>): void
  onSelect?(e: SyntheticEvent<HTMLTextAreaElement>): void
  onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>): void
  onPaste(e: ClipboardEvent<HTMLTextAreaElement>): void
}

// The textarea's own text is transparent; a mirror behind it, in the same font, width and wrapping, shows the
// markdown in colour, so the caret and the selection stay the textarea's own.
export function ComposerEditor({ taRef, value, label, placeholder, onChange, onSelect, onKeyDown, onPaste }: ComposerEditorProps) {
  const mirror = useRef<HTMLDivElement>(null)
  const [composing, setComposing] = useState(false)
  const spans = useMemo(() => markdownSpans(value), [value])

  useLayoutEffect(() => {
    if (mirror.current && taRef.current) mirror.current.scrollTop = taRef.current.scrollTop
  }, [value, taRef])

  return (
    <div className="composer-input text-body" data-composing={composing}>
      <div ref={mirror} className="composer-mirror" aria-hidden="true">
        {spans.map((span, i) => (span.kind ? <span key={i} className={`md-${span.kind}`}>{span.text}</span> : span.text))}
        {'​'}
      </div>
      <AutoTextarea
        taRef={taRef}
        value={value}
        aria-label={label}
        placeholder={placeholder}
        className="composer-editor block"
        onChange={onChange}
        onSelect={onSelect}
        onKeyDown={onKeyDown}
        onPaste={onPaste}
        onScroll={(e) => {
          if (mirror.current) mirror.current.scrollTop = e.currentTarget.scrollTop
        }}
        onCompositionStart={() => setComposing(true)}
        onCompositionEnd={() => setComposing(false)}
      />
    </div>
  )
}
