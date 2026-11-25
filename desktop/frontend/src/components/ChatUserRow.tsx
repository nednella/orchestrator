import { memo } from 'react'
import { ChatMessageActions } from './ChatMessageActions'

type ChatUserRowProps = { text: string; onReply(text: string): void }

export const ChatUserRow = memo(function ChatUserRow({ text, onReply }: ChatUserRowProps) {
  return (
    <div data-message className="group/message relative -mb-3 flex justify-end pb-6">
      <p className="max-w-[85%] rounded-lg bg-raised px-3.5 py-2 text-body leading-[1.55] break-words whitespace-pre-wrap select-text">{text}</p>
      <ChatMessageActions text={text} onReply={onReply} align="end" />
    </div>
  )
})
