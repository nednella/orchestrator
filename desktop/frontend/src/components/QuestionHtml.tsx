import { useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { previewFrame, previewHeight, previewWidth } from '../questionPreview'
import { useElementWidth } from '../useElementWidth'
import { usePreviewTokens } from '../usePreviewTokens'
import { QuestionHtmlDialog } from './QuestionHtmlDialog'

type QuestionHtmlProps = { html: string }

// QuestionHtml is an html block of a preview. The page is laid out at a fixed width and scaled down to its tile; the
// frame gets no events, so a button above it opens the page at its full size.
export function QuestionHtml({ html }: QuestionHtmlProps) {
  const box = useRef<HTMLDivElement>(null)
  const width = useElementWidth(box)
  const tokens = usePreviewTokens()
  const [open, setOpen] = useState(false)
  const scale = width > 0 ? width / previewWidth : 0

  return (
    <div ref={box} className="q-html" style={{ height: previewHeight * scale }}>
      {scale > 0 && <iframe title="Preview" tabIndex={-1} {...previewFrame(html, tokens)} style={{ width: previewWidth, height: previewHeight, transform: `scale(${scale})` }} />}
      <button type="button" className="q-html-open" aria-label="Open preview full size" onClick={() => setOpen(true)} />
      {open && createPortal(<QuestionHtmlDialog html={html} tokens={tokens} onClose={() => setOpen(false)} />, document.body)}
    </div>
  )
}
