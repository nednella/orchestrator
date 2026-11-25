import { memo } from 'react'
import { ChatMarkdown } from './ChatMarkdown'
import { ChatMessageActions } from './ChatMessageActions'

type ChatReplyRowProps = { uuid: string; text: string; streaming: boolean; onReply(text: string): void; onFork(at: string, draft: string): void }

export const ChatReplyRow = memo(function ChatReplyRow({ uuid, text, streaming, onReply, onFork }: ChatReplyRowProps) {
  return (
    <div data-message data-streaming={streaming || undefined} className="group/message relative -mb-3 pb-6">
      <ChatMarkdown text={text} />
      {!streaming && <ChatMessageActions text={text} onReply={onReply} onFork={uuid ? () => onFork(uuid, '') : undefined} align="start" />}
    </div>
  )
})
