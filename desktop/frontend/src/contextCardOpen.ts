import { useEffect, useLayoutEffect, useSyncExternalStore } from 'react'
import type { RefObject } from 'react'
import { cardPlacement } from './contextCard'
import { readStored, writeStored } from './storage'

const SHOWN_KEY = 'agentos.contextCard'

// shown is the owner's saved choice for the card over the chat; popover is the card opened under its top-bar button,
// while the chat is too small to float it, and it is never saved.
type CardOpen = { shown: boolean; popover: boolean; placement: 'inline' | 'popover' }

let card: CardOpen = { shown: readStored<unknown>(SHOWN_KEY, true) !== false, popover: false, placement: 'inline' }
const listeners = new Set<() => void>()

function update(next: Partial<CardOpen>) {
  card = { ...card, ...next }
  listeners.forEach((listener) => listener())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function useContextCard(): CardOpen {
  return useSyncExternalStore(subscribe, () => card)
}

// toggleContextCard shows or hides the card where it is now: over the chat, or in its popover.
export function toggleContextCard() {
  if (card.placement === 'popover') {
    update({ popover: !card.popover })
    return
  }
  writeStored(SHOWN_KEY, !card.shown)
  update({ shown: !card.shown })
}

export function closeCardPopover() {
  if (card.popover) update({ popover: false })
}

// useCardPlacement measures the chat area and says where the card goes in it; a popover left open closes once the
// card fits over the chat again.
export function useCardPlacement(area: RefObject<HTMLElement | null>, mounted: boolean) {
  useLayoutEffect(() => {
    const el = area.current
    if (!el) return
    const measure = () => {
      const placement = cardPlacement(el.clientWidth, el.clientHeight)
      if (placement !== card.placement) update({ placement, popover: false })
    }
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(el)
    return () => observer.disconnect()
  }, [area, mounted])
}

// useCardPopoverDismiss closes the popover at a click outside root, which holds its button and the popover, or at Esc.
export function useCardPopoverDismiss(root: RefObject<HTMLElement | null>, open: boolean) {
  useEffect(() => {
    if (!open) return
    const outside = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) closeCardPopover()
    }
    const escape = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopPropagation()
      closeCardPopover()
    }
    window.addEventListener('mousedown', outside)
    window.addEventListener('keydown', escape, true)
    return () => {
      window.removeEventListener('mousedown', outside)
      window.removeEventListener('keydown', escape, true)
    }
  }, [root, open])
}
