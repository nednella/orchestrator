import { useEffect, useRef, useState } from 'react'
import { applyAccent } from '../accent'
import { hexToHsv, hsvToHex } from '../accentWheel'
import type { Hsv } from '../accentWheel'
import type { Appearance } from '../themes'
import type { ThemeName } from '../types'
import { useColorScheme } from '../useColorScheme'
import { AccentWheelPopover } from './AccentWheelPopover'

// saved is the accent in the config, custom the part of it that is a custom color ('' while it is not one), and start
// the color the wheel opens on without one.
type AccentCustomSwatchProps = { mode: Appearance; theme: ThemeName; saved: string; custom: string; start: string; onPick(accent: string): void }

// AccentCustomSwatch is the choice of any color: a disc of every hue, or the custom color once there is one. It opens
// the wheel, which shows its color on the page as it moves; only the color kept is picked.
export function AccentCustomSwatch({ mode, theme, saved, custom, start, onPick }: AccentCustomSwatchProps) {
  const button = useRef<HTMLButtonElement>(null)
  const [anchor, setAnchor] = useState<DOMRect | null>(null)
  const [color, setColor] = useState<Hsv>(() => hexToHsv(custom || start))
  const scheme = useColorScheme()
  const picked = custom !== ''
  const open = anchor !== null
  const shown = scheme === mode

  const onPage = (accent: string) => {
    if (shown) applyAccent(document.documentElement.style, accent)
  }

  const close = (refocus: boolean) => {
    setAnchor(null)
    if (refocus) button.current?.focus()
  }

  const keep = () => {
    const hex = hsvToHex(color)
    close(true)
    if (hex !== saved) onPick(hex)
  }

  const revert = () => {
    onPage(saved)
    close(true)
  }

  // A page that goes while the wheel is open leaves the accent it was showing as it was saved.
  const showing = useRef(false)
  useEffect(() => {
    showing.current = open && shown
  }, [open, shown])
  useEffect(
    () => () => {
      if (showing.current) applyAccent(document.documentElement.style, saved)
    },
    [saved],
  )

  return (
    <>
      <button
        ref={button}
        type="button"
        title="Custom"
        aria-label={`Custom accent for ${mode} mode`}
        aria-pressed={picked}
        aria-haspopup="dialog"
        aria-expanded={open}
        className="relative flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-full p-1 outline-none focus-visible:outline-2 focus-visible:outline-ink/60 focus-visible:outline-solid"
        onClick={() => {
          if (open) return keep()
          setColor(hexToHsv(custom || start))
          setAnchor(button.current?.getBoundingClientRect() ?? null)
        }}
      >
        <span
          aria-hidden="true"
          className="block size-full rounded-full"
          style={{ backgroundColor: picked ? custom : undefined, backgroundImage: picked ? undefined : 'var(--accent-wheel-hues)', boxShadow: `var(--shadow-preview-${mode})` }}
        />
        {picked && <span aria-hidden="true" className="pointer-events-none absolute inset-0 rounded-full ring-2 ring-ink ring-inset" />}
      </button>
      {anchor && (
        <AccentWheelPopover
          anchor={anchor}
          trigger={button}
          theme={theme}
          color={color}
          onChange={(next) => {
            setColor(next)
            onPage(hsvToHex(next))
          }}
          onKeep={keep}
          onRevert={revert}
        />
      )}
    </>
  )
}
