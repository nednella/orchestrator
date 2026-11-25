import { useEffect, useState } from 'react'
import { ChevronDown } from 'lucide-react'

// It shows a moment after the chat stops following its end, so that it does not flash as a thread opens.
const SHOW_DELAY = 150

// shown is whether the chat is scrolled away from its end; queued counts the messages Claude has not taken in yet.
type ChatEndPillProps = { shown: boolean; queued: number; onClick(): void }

export function ChatEndPill({ shown, queued, onClick }: ChatEndPillProps) {
  const [settled, setSettled] = useState(false)

  useEffect(() => {
    setSettled(false)
    if (!shown) return
    const timer = setTimeout(() => setSettled(true), SHOW_DELAY)
    return () => clearTimeout(timer)
  }, [shown])

  if (!shown || !settled) return null
  return (
    <div className="pointer-events-none absolute inset-x-0 bottom-full z-30 flex justify-center pb-1">
      <button className="chat-end-pill glass fade-in" onMouseDown={(e) => e.preventDefault()} onClick={onClick}>
        <ChevronDown size={14} className="text-soft" aria-hidden="true" />
        Scroll to end
        {queued > 0 && <span className="text-soft">· {queued} queued</span>}
      </button>
    </div>
  )
}
