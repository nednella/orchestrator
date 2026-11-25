import { useCopied } from '../useCopied'

type ChatMessageActionsProps = { text: string; onReply(text: string): void; align: 'start' | 'end' }

export function ChatMessageActions({ text, onReply, align }: ChatMessageActionsProps) {
  const [copied, copy] = useCopied(text)

  return (
    <div
      className={`absolute top-full -mt-5 flex gap-0.5 opacity-0 transition-opacity group-hover/message:opacity-100 group-focus-within/message:opacity-100 ${align === 'end' ? 'right-0' : 'left-0'}`}
    >
      <button className="btn btn-ghost h-6 px-1.5" onClick={copy}>
        {copied ? 'Copied' : 'Copy'}
      </button>
      <button className="btn btn-ghost h-6 px-1.5" onClick={() => onReply(text)}>
        Reply
      </button>
    </div>
  )
}
