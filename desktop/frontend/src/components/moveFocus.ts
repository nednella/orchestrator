import type { KeyboardEvent } from 'react'

// moveFocus moves the focus to the previous or next enabled element that matches the selector, inside the element that
// has the key handler, on the arrow keys Up and Down. It reports whether it took the key.
export function moveFocus(e: KeyboardEvent<HTMLElement>, selector: string, keys: [string, string] = ['ArrowUp', 'ArrowDown']): boolean {
  const step = e.key === keys[1] ? 1 : e.key === keys[0] ? -1 : 0
  if (step === 0) return false
  e.preventDefault()
  const items = [...e.currentTarget.querySelectorAll<HTMLElement>(`${selector}:not(:disabled)`)]
  const i = items.indexOf(document.activeElement as HTMLElement)
  items[Math.max(0, Math.min(i + step, items.length - 1))]?.focus()
  return true
}
