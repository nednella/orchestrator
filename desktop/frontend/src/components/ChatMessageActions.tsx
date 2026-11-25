import { GitFork, Reply } from 'lucide-react'
import { useState } from 'react'
import type { TurnEnd } from '../chatTimeline'
import { ChatActionButton, revealOnHover } from './ChatActionButton'
import { ChatCopyButton } from './ChatCopyButton'
import { ChatMessageTime } from './ChatMessageTime'

const endedLabel: Record<TurnEnd, string> = { stopped: 'Stopped', failed: 'Failed' }

// The actions under a reply. The row keeps its height while the reply streams, so that it does not move the thread when
// the reply ends. Under tool rows that follow the reply it is always shown. onFork is left out for a reply the thread
// cannot be forked from.
type ChatMessageActionsProps = { text: string; at: number; streaming: boolean; ended?: TurnEnd; shown?: boolean; onReply(text: string): void; onFork?(): Promise<void> }

export function ChatMessageActions({ text, at, streaming, ended, shown = false, onReply, onFork }: ChatMessageActionsProps) {
  const [forking, setForking] = useState(false)

  const fork = () => {
    if (!onFork || forking) return
    setForking(true)
    onFork().finally(() => setForking(false))
  }

  return (
    <div className={`mt-1.5 flex h-7 items-center gap-2 text-small tabular-nums select-none sm:h-6 ${shown ? '' : revealOnHover}`}>
      {ended && <span className="rounded-full border border-[color-mix(in_srgb,var(--border)_70%,transparent)] px-1.5 py-0.5 font-mono text-[0.625rem] leading-none text-soft">{endedLabel[ended]}</span>}
      {!streaming && <ChatCopyButton text={text} />}
      {onFork && (
        <ChatActionButton label="Fork from this response" busy={forking} onClick={fork}>
          <GitFork className="size-3" />
        </ChatActionButton>
      )}
      {!streaming && (
        <>
          <ChatActionButton label="Reply to this response" muted onClick={() => onReply(text)}>
            <Reply className="size-3" />
          </ChatActionButton>
          <ChatMessageTime at={at} />
        </>
      )}
    </div>
  )
}
