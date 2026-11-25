import { memo } from 'react'
import { ChatMarkdown } from './ChatMarkdown'
import { ChatMessageActions } from './ChatMessageActions'

type ChatReplyRowProps = { uuid: string; text: string; at: number; streaming: boolean; onQuote(text: string): void; onFork(at: string, draft: string): void }

export const ChatReplyRow = memo(function ChatReplyRow({ uuid, text, at, streaming, onQuote, onFork }: ChatReplyRowProps) {
  return (
    <div data-message data-streaming={streaming || undefined} className="group/message -mb-3">
      <ChatMarkdown text={text} />
      <ChatMessageActions text={text} at={at} streaming={streaming} onQuote={onQuote} onFork={uuid ? () => onFork(uuid, '') : undefined} />
    </div>
  )
})
