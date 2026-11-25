import { Brain } from 'lucide-react'
import { memo } from 'react'
import { thoughtLine } from '../chatThought'
import { ChatShimmer } from './ChatShimmer'

type ChatThoughtRowProps = { text: string; live: boolean }

// ChatThoughtRow is one thinking block of a turn as a line: its heading or first sentence, or "Thinking" while it has
// none, shimmering while it streams.
export const ChatThoughtRow = memo(function ChatThoughtRow({ text, live }: ChatThoughtRowProps) {
  const line = thoughtLine(text)
  if (!line && !live) return null
  const label = line || 'Thinking'
  return (
    <div className="flex min-w-0 items-start gap-2 py-1 text-small text-soft select-none">
      <span className="flex h-[1lh] flex-none items-center text-dim">
        <Brain size={12} strokeWidth={2} aria-hidden />
      </span>
      {live ? <ChatShimmer text={label} /> : <span className="line-clamp-4 min-w-0">{label}</span>}
    </div>
  )
})
