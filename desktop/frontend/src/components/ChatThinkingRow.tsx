import { Brain } from 'lucide-react'
import { memo } from 'react'
import { ChatShimmer } from './ChatShimmer'

type ChatThinkingRowProps = { label: string }

// ChatThinkingRow ends the turn that runs while nothing in it streams, so it sits just above the composer.
export const ChatThinkingRow = memo(function ChatThinkingRow({ label }: ChatThinkingRowProps) {
  return (
    <div className="flex min-w-0 items-start gap-2 py-1 text-small text-soft select-none">
      <span className="flex h-[1lh] flex-none items-center text-soft/70">
        <Brain size={12} strokeWidth={2} aria-hidden />
      </span>
      <ChatShimmer text={label} />
    </div>
  )
})
