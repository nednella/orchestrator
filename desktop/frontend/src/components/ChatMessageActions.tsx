import { useCopied } from '../useCopied'

// onFork is left out for a message the thread cannot be forked from.
type ChatMessageActionsProps = { text: string; onReply(text: string): void; onFork?(): void; align: 'start' | 'end' }

export function ChatMessageActions({ text, onReply, onFork, align }: ChatMessageActionsProps) {
  const [copied, copy] = useCopied(text)

  return (
    <div
      className={`absolute bottom-0 flex select-none gap-0.5 opacity-0 transition-opacity group-hover/message:opacity-100 group-focus-within/message:opacity-100 ${align === 'end' ? 'right-0' : 'left-0'}`}
    >
      <button className="btn btn-ghost h-6 px-1.5" onClick={copy}>
        {copied ? 'Copied' : 'Copy'}
      </button>
      <button className="btn btn-ghost h-6 px-1.5" onClick={() => onReply(text)}>
        Reply
      </button>
      {onFork && (
        <button className="btn btn-ghost h-6 px-1.5" onClick={onFork}>
          Fork from here
        </button>
      )}
    </div>
  )
}
