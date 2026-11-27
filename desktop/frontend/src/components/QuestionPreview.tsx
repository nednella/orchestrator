import { memo } from 'react'
import type { PreviewPart } from '../questionPreview'
import { ChatMarkdown } from './ChatMarkdown'
import { QuestionHtml } from './QuestionHtml'
import { QuestionImage } from './QuestionImage'

type QuestionPreviewProps = { thread: string; parts: PreviewPart[] }

// QuestionPreview is what an option shows itself by: code that keeps its spaces, markdown, images of the thread and
// html pages, in order.
export const QuestionPreview = memo(function QuestionPreview({ thread, parts }: QuestionPreviewProps) {
  return (
    <div className="q-preview">
      {parts.map((part, i) => {
        switch (part.kind) {
          case 'html':
            return <QuestionHtml key={i} html={part.html} />
          case 'image':
            return <QuestionImage key={i} thread={thread} src={part.src} alt={part.alt} />
          case 'code':
            return (
              <pre key={i} className="q-code">
                {part.text}
              </pre>
            )
          case 'markdown':
            return <ChatMarkdown key={i} text={part.text} />
        }
      })}
    </div>
  )
})
