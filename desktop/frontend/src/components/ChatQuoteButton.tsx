import { useEffect, useState } from 'react'

type ChatQuoteButtonProps = { onQuote(text: string): void }
type Selected = { text: string; x: number; y: number }

const messageOf = (node: Node | null) => (node instanceof Element ? node : node?.parentElement)?.closest('[data-message]') ?? null

// Half the button's width, and the space the button keeps from the edges of the chat.
const half = 36
const edge = 8

function selected(): Selected | null {
  const selection = document.getSelection()
  if (!selection || selection.rangeCount === 0 || selection.isCollapsed) return null
  const range = selection.getRangeAt(0)
  const message = messageOf(range.startContainer)
  // A triple click ends at the start of the next block, with none of it selected.
  if (!message || (range.endOffset > 0 && message !== messageOf(range.endContainer))) return null
  const text = selection.toString()
  const area = message.closest('[data-chat-scroll]')?.getBoundingClientRect()
  if (text.trim() === '' || !area) return null
  const rect = range.getBoundingClientRect()
  return {
    text,
    x: Math.min(Math.max(rect.left + rect.width / 2, area.left + half + edge), area.right - half - edge),
    y: Math.min(Math.max(rect.top - 6, area.top + 36), area.bottom - edge),
  }
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
      style={{ left: current.x, top: current.y }}
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
