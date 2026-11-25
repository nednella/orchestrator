import { messageTime, messageTimeTitle } from '../messageTime'
import { useClock } from '../time'

type ChatMessageTimeProps = { at: number }

export function ChatMessageTime({ at }: ChatMessageTimeProps) {
  const text = useClock((now) => messageTime(at, now))
  return (
    <p className="flex-none text-small leading-4 text-soft tabular-nums" title={messageTimeTitle(at)}>
      {text}
    </p>
  )
}
