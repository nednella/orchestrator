import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { KeyboardEvent, RefObject } from 'react'
import { createPortal } from 'react-dom'
import { keepFocusIn } from '../focusTrap'
import { hexToHsv, hsvToHex, normalizeHex } from '../accentWheel'
import type { Hsv } from '../accentWheel'
import type { ThemeName } from '../types'
import { AccentBrightness } from './AccentBrightness'
import { AccentPreview } from './AccentPreview'
import { AccentWheel } from './AccentWheel'

type AccentWheelPopoverProps = {
  anchor: DOMRect
  trigger: RefObject<HTMLButtonElement | null>
  theme: ThemeName
  color: Hsv
  onChange(color: Hsv): void
  onKeep(): void
  onRevert(): void
}

const GAP_PX = 6
const EDGE_PX = 8

// AccentWheelPopover opens beside the custom swatch with its right edges level, over it when there is no room under it.
// A click outside or Enter keeps the color; Esc gives it up.
export function AccentWheelPopover({ anchor, trigger, theme, color, onChange, onKeep, onRevert }: AccentWheelPopoverProps) {
  const popup = useRef<HTMLDivElement>(null)
  const [place, setPlace] = useState<{ left: number; top: number } | null>(null)
  // What the hex field holds while it is being typed in; null shows the color's own.
  const [typed, setTyped] = useState<string | null>(null)
  const hex = hsvToHex(color)
  const invalid = typed !== null && normalizeHex(typed) === null

  useLayoutEffect(() => {
    const el = popup.current
    if (!el) return
    const { width, height } = el.getBoundingClientRect()
    const left = Math.max(EDGE_PX, Math.min(anchor.right - width, window.innerWidth - width - EDGE_PX))
    const below = anchor.bottom + GAP_PX + height <= window.innerHeight - EDGE_PX
    setPlace({ left, top: below ? anchor.bottom + GAP_PX : Math.max(EDGE_PX, anchor.top - GAP_PX - height) })
  }, [anchor])

  // The popup is hidden until it has its place, and a hidden element takes no focus.
  const placed = place !== null
  useEffect(() => {
    if (placed) popup.current?.querySelector<HTMLElement>('[role="slider"]')?.focus()
  }, [placed])

  const latest = useRef({ onKeep, onRevert })
  useLayoutEffect(() => {
    latest.current = { onKeep, onRevert }
  })
  useEffect(() => {
    const outside = (e: MouseEvent) => {
      const target = e.target as Node
      if (!popup.current?.contains(target) && !trigger.current?.contains(target)) latest.current.onKeep()
    }
    const escape = (e: globalThis.KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopPropagation()
      latest.current.onRevert()
    }
    const moved = (e: Event) => {
      if (!popup.current?.contains(e.target as Node)) latest.current.onKeep()
    }
    const resized = () => latest.current.onKeep()
    window.addEventListener('mousedown', outside)
    window.addEventListener('keydown', escape, true)
    window.addEventListener('scroll', moved, true)
    window.addEventListener('resize', resized)
    return () => {
      window.removeEventListener('mousedown', outside)
      window.removeEventListener('keydown', escape, true)
      window.removeEventListener('scroll', moved, true)
      window.removeEventListener('resize', resized)
    }
  }, [trigger])

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Tab') keepFocusIn(popup.current, e.nativeEvent)
    else if (e.key === 'Enter' && !invalid) {
      e.preventDefault()
      onKeep()
    }
  }

  return createPortal(
    <div
      ref={popup}
      role="dialog"
      aria-label="Custom accent"
      className="fade-in fixed grid w-60 gap-3 rounded-lg border border-(--border-glass) bg-(--bg-dropdown) bg-(image:--bg-dropdown-tint) p-3 shadow-(--shadow-popup) backdrop-blur-[16px] backdrop-saturate-[1.08]"
      style={{ zIndex: 'var(--z-select)', ...(place ?? { left: anchor.left, top: anchor.bottom, visibility: 'hidden' }) }}
      onKeyDown={onKeyDown}
    >
      <AccentWheel color={color} onChange={onChange} />
      <AccentBrightness color={color} onChange={onChange} />
      <div className="grid gap-1">
        <input
          type="text"
          aria-label="Hex color"
          aria-invalid={invalid}
          aria-describedby={invalid ? 'accent-hex-hint' : undefined}
          spellCheck={false}
          autoComplete="off"
          maxLength={7}
          value={typed ?? hex}
          className={`setting-field h-8 w-full px-2.5 font-mono ${invalid ? 'border-(--danger)' : ''}`}
          onChange={(e) => {
            setTyped(e.target.value)
            const next = normalizeHex(e.target.value)
            if (next) onChange(hexToHsv(next))
          }}
          onBlur={() => setTyped(null)}
        />
        {invalid && (
          <span id="accent-hex-hint" className="text-small text-(--danger)">
            Write the color as #rrggbb.
          </span>
        )}
      </div>
      <AccentPreview theme={theme} accent={hex} />
    </div>,
    document.body,
  )
}
