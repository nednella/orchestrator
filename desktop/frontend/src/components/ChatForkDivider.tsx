import { GitFork } from 'lucide-react'
import { memo } from 'react'
import { useOrchestrator } from '../OrchestratorContext'

type ChatForkDividerProps = { thread: string; label: string; hint: string }

export const ChatForkDivider = memo(function ChatForkDivider({ thread, label, hint }: ChatForkDividerProps) {
  const { select } = useOrchestrator('select')
  return (
    <div className="flex items-center gap-2 py-2 text-[11px] text-dim">
      <span className="h-px flex-1 bg-(--border-fork-divider)" />
      <button type="button" title={hint} onClick={() => select(thread)} className="flex cursor-pointer items-center gap-1 rounded-full border border-(--border-fork-divider) bg-surface px-2.5 py-1 hover:bg-accent hover:text-(--text-on-accent) focus-visible:ring-2 focus-visible:ring-accent focus-visible:outline-none focus-visible:ring-inset">
        <GitFork className="size-3" />
        {label}
      </button>
      <span className="h-px flex-1 bg-(--border-fork-divider)" />
    </div>
  )
})
