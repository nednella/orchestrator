import { useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

const EDGE_PX = 4
const GAP_PX = 6

type FloatingTipProps = { anchor: DOMRect; text: string; below?: boolean }

// FloatingTip shows text centred above anchor, or below it, kept inside the window, over whatever clips the element it names.
export function FloatingTip({ anchor, text, below = false }: FloatingTipProps) {
  const tip = useRef<HTMLSpanElement>(null)
  const [place, setPlace] = useState<{ left: number; top: number } | null>(null)

  useLayoutEffect(() => {
    const el = tip.current
    if (!el) return
    const { width, height } = el.getBoundingClientRect()
    const left = Math.max(EDGE_PX, Math.min(anchor.left + anchor.width / 2 - width / 2, window.innerWidth - width - EDGE_PX))
    const top = below ? Math.min(anchor.bottom + GAP_PX, window.innerHeight - height - EDGE_PX) : Math.max(EDGE_PX, anchor.top - height - GAP_PX)
    setPlace({ left, top })
  }, [anchor, text, below])

  return createPortal(
    <span
      ref={tip}
      aria-hidden="true"
      className="pointer-events-none fixed rounded-md bg-[var(--bg-glass)] px-2 py-1 text-small whitespace-nowrap text-ink shadow-[var(--shadow-tip)] backdrop-blur-[16px] backdrop-saturate-[1.08]"
      style={{ zIndex: 'var(--z-tip)', ...(place ?? { left: anchor.left, top: anchor.top, opacity: 0 }) }}
    >
      {text}
    </span>,
    document.body,
  )
}
