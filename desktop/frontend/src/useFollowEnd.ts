import { useLayoutEffect, useRef } from 'react'
import type { RefObject } from 'react'
import { stillFollows } from './chatScroll'

// useFollowEnd keeps a plain scroller at the end of its content as the content grows, while the reader is there, by
// the chat's rule for when it follows. It opens at the end. The returned function is the scroller's onScroll.
export function useFollowEnd(scroller: RefObject<HTMLElement | null>, content: RefObject<HTMLElement | null>): () => void {
  const following = useRef(true)
  const lastTop = useRef(0)

  useLayoutEffect(() => {
    const el = scroller.current
    const inner = content.current
    if (!el || !inner) return
    const toEnd = () => {
      if (following.current) el.scrollTop = el.scrollHeight
    }
    toEnd()
    const observer = new ResizeObserver(toEnd)
    observer.observe(el)
    observer.observe(inner)
    return () => observer.disconnect()
  }, [scroller, content])

  return () => {
    const el = scroller.current
    if (!el) return
    following.current = stillFollows({ following: following.current, top: el.scrollTop, lastTop: lastTop.current, client: el.clientHeight, height: el.scrollHeight })
    lastTop.current = el.scrollTop
  }
}
