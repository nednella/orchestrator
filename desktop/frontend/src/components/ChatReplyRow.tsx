import { memo } from 'react'
import type { TurnEnd } from '../chatTimeline'
import { ChatMarkdown } from './ChatMarkdown'
import { ChatMessageActions } from './ChatMessageActions'

// toolsAfter says tool rows follow the reply, under which its actions show instead.
type ChatReplyRowProps = {
  uuid: string
  text: string
  at: number
  streaming: boolean
  ended?: TurnEnd
  toolsAfter: boolean
  onReply(text: string): void
  onFork?(at: string, draft: string): Promise<void>
}

export const ChatReplyRow = memo(function ChatReplyRow({ uuid, text, at, streaming, ended, toolsAfter, onReply, onFork }: ChatReplyRowProps) {
  return (
    <div data-message data-streaming={streaming || undefined} className="group/message -mb-3">
      <ChatMarkdown text={text} streaming={streaming} />
      {!toolsAfter && <ChatMessageActions text={text} at={at} streaming={streaming} ended={ended} onReply={onReply} onFork={uuid && onFork ? () => onFork(uuid, '') : undefined} />}
    </div>
  )
})
