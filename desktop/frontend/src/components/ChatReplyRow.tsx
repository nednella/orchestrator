import { memo } from 'react'
import { ChatMarkdown } from './ChatMarkdown'

type ChatReplyRowProps = { text: string; streaming: boolean }

export const ChatReplyRow = memo(function ChatReplyRow({ text, streaming }: ChatReplyRowProps) {
  return (
    <div data-streaming={streaming || undefined}>
      <ChatMarkdown text={text} />
    </div>
  )
})
