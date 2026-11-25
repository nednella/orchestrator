import { useEffect, useState } from 'react'

type ChatQuoteButtonProps = { onQuote(text: string): void }
type Selected = { text: string; x: number; y: number }

const messageOf = (node: Node | null) => (node instanceof Element ? node : node?.parentElement)?.closest('[data-message]') ?? null

function selected(): Selected | null {
  const selection = document.getSelection()
  if (!selection || selection.rangeCount === 0 || selection.isCollapsed) return null
  const message = messageOf(selection.anchorNode)
  if (!message || message !== messageOf(selection.focusNode)) return null
  const text = selection.toString()
  if (text.trim() === '') return null
  const rect = selection.getRangeAt(0).getBoundingClientRect()
  return { text, x: rect.left + rect.width / 2, y: rect.top }
}

// A button over a selection that lies in one chat message.
export function ChatQuoteButton({ onQuote }: ChatQuoteButtonProps) {
  const [current, setCurrent] = useState<Selected | null>(null)

  useEffect(() => {
    const update = () => setCurrent(selected())
    document.addEventListener('selectionchange', update)
    // The chat scrolls beneath the selection.
    document.addEventListener('scroll', update, true)
    return () => {
      document.removeEventListener('selectionchange', update)
      document.removeEventListener('scroll', update, true)
    }
  }, [])

  if (!current) return null

  return (
    <button
      className="btn chat-jump fixed z-20 -translate-x-1/2 -translate-y-full"
      style={{ left: current.x, top: Math.max(current.y - 6, 40) }}
      // A press must not clear the selection.
      onMouseDown={(e) => e.preventDefault()}
      onClick={() => {
        document.getSelection()?.removeAllRanges()
        onQuote(current.text)
      }}
    >
      Quote
    </button>
  )
}
