import { memo } from 'react'
import { ChatMarkdown } from './ChatMarkdown'
import { ChatMessageActions } from './ChatMessageActions'

type ChatReplyRowProps = { text: string; streaming: boolean; onReply(text: string): void }

export const ChatReplyRow = memo(function ChatReplyRow({ text, streaming, onReply }: ChatReplyRowProps) {
  return (
    <div data-message data-streaming={streaming || undefined} className="group/message relative -mb-5 pb-5">
      <ChatMarkdown text={text} />
      {!streaming && <ChatMessageActions text={text} onReply={onReply} align="start" />}
    </div>
  )
})
