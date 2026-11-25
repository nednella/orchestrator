import { Check, Copy } from 'lucide-react'
import { useCopied } from '../useCopied'
import { ChatActionButton } from './ChatActionButton'

type ChatCopyButtonProps = { text: string }

export function ChatCopyButton({ text }: ChatCopyButtonProps) {
  const [copied, copy] = useCopied(text, 1000)
  return (
    <ChatActionButton label={copied ? 'Copied' : 'Copy message'} muted disabled={copied} onClick={copy}>
      {copied ? <Check className="size-3 text-accent" /> : <Copy className="size-3" />}
    </ChatActionButton>
  )
}
