import { Bot } from 'lucide-react'

type ChatLineageDividerProps = { parent: string; onOpenParent(): void }

// ChatLineageDivider heads the chat of an agent another thread started, with the way back to that thread.
export function ChatLineageDivider({ parent, onOpenParent }: ChatLineageDividerProps) {
  return (
    <div className="flex items-center gap-2 py-2 text-label text-dim">
      <span className="h-px flex-1 bg-(--border-fork-divider)" />
      <button type="button" title="Open parent thread" onClick={onOpenParent} className="flex min-w-0 cursor-pointer items-center gap-1 rounded-full border border-(--border-fork-divider) bg-surface px-2.5 py-1 hover:bg-accent hover:text-(--text-on-accent) focus-visible:ring-2 focus-visible:ring-accent focus-visible:outline-none focus-visible:ring-inset">
        <Bot className="size-3 flex-none" aria-hidden="true" />
        <span className="truncate">Subagent of {parent}</span>
      </button>
      <span className="h-px flex-1 bg-(--border-fork-divider)" />
    </div>
  )
}
