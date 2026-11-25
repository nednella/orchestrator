import { GitFork, Reply } from 'lucide-react'
import { ChatActionButton, revealOnHover } from './ChatActionButton'
import { ChatCopyButton } from './ChatCopyButton'
import { ChatMessageTime } from './ChatMessageTime'

// The actions under a reply. The row keeps its height while the reply streams, so that it does not move the thread when
// the reply ends. onFork is left out for a reply the thread cannot be forked from.
type ChatMessageActionsProps = { text: string; at: number; streaming: boolean; onQuote(text: string): void; onFork?(): void }

export function ChatMessageActions({ text, at, streaming, onQuote, onFork }: ChatMessageActionsProps) {
  return (
    <div className={`mt-1.5 flex h-7 items-center gap-2 select-none sm:h-6 ${revealOnHover}`}>
      {!streaming && (
        <>
          <ChatCopyButton text={text} />
          {onFork && (
            <ChatActionButton label="Fork from this response" onClick={onFork}>
              <GitFork className="size-3" />
            </ChatActionButton>
          )}
          <ChatActionButton label="Reply to this response" muted onClick={() => onQuote(text)}>
            <Reply className="size-3" />
          </ChatActionButton>
          <ChatMessageTime at={at} />
        </>
      )}
    </div>
  )
}
