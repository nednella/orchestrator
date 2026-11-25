import { useLayoutEffect, useRef, useState } from 'react'
import { fadeHeight, foldHeight, folds } from '../messageFold'
import { ChatActionButton } from './ChatActionButton'

// id is the message's own, '' when it has none.
type ChatUserTextProps = { id: string; text: string }

const folded = { maxHeight: `${foldHeight}rem`, maskImage: `linear-gradient(to bottom, #000 calc(100% - ${fadeHeight}rem), transparent)` }

// The messages opened in full, kept while the app runs: a row out of view is let go and made again when it comes back.
const opened = new Set<string>()

// The text of the owner's message, folded when it is long.
export function ChatUserText({ text, id }: ChatUserTextProps) {
  const body = useRef<HTMLParagraphElement>(null)
  const [long, setLong] = useState(false)
  const [open, setOpen] = useState(() => opened.has(id))

  useLayoutEffect(() => {
    const el = body.current
    if (!el) return
    const measure = () => setLong(folds(el.scrollHeight, parseFloat(getComputedStyle(document.documentElement).fontSize)))
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(el)
    return () => observer.disconnect()
  }, [text])

  const toggle = () => {
    if (open) opened.delete(id)
    else if (id) opened.add(id)
    setOpen(!open)
  }

  return (
    <>
      <p ref={body} className="overflow-hidden text-body leading-[1.55] break-words whitespace-pre-wrap select-text" style={long && !open ? folded : undefined}>
        {text}
      </p>
      {long && (
        <div className="mt-1.5 -ml-2 select-none">
          <ChatActionButton muted onClick={toggle}>
            {open ? 'Show less' : 'Show full message'}
          </ChatActionButton>
        </div>
      )}
    </>
  )
}
