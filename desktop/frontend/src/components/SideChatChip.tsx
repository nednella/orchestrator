import { MessagesSquare, X } from 'lucide-react'

type SideChatChipProps = { title: string; onOpen(): void; onRemove(): void }

// SideChatChip is a side chat added to the thread's next message, in the composer: it opens the side chat's tab, and
// its x takes it off the message.
export function SideChatChip({ title, onOpen, onRemove }: SideChatChipProps) {
  return (
    <span className="inline-flex h-6 max-w-56 min-w-0 flex-none items-center gap-1 rounded-md border border-line-strong bg-raised pr-0.5 pl-1.5 text-small font-medium text-soft">
      <button type="button" className="flex min-w-0 items-center gap-1 hover:text-ink" title={`Side chat: ${title}`} onMouseDown={(e) => e.preventDefault()} onClick={onOpen}>
        <MessagesSquare size={12} className="flex-none" aria-hidden="true" />
        <span className="min-w-0 truncate">{title}</span>
      </button>
      <button type="button" className="grid size-4 flex-none place-items-center rounded-sm hover:bg-(--bg-segment-on) hover:text-ink" aria-label={`Remove side chat ${title}`} onMouseDown={(e) => e.preventDefault()} onClick={onRemove}>
        <X size={10} aria-hidden="true" />
      </button>
    </span>
  )
}
