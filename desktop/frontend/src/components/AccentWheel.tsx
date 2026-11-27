import { useRef } from 'react'
import type { KeyboardEvent, PointerEvent } from 'react'
import { hsvToHex, nudge, pointToHueSaturation, wheelPoint } from '../accentWheel'
import type { Hsv } from '../accentWheel'

type AccentWheelProps = { color: Hsv; onChange(color: Hsv): void }

const STEP = 0.02
const BIG_STEP = 0.1

// AccentWheel is the hue round the circle and the saturation from its centre out, with a handle at the color. The
// brightness is the slider's, and darkens the wheel as it falls.
export function AccentWheel({ color, onChange }: AccentWheelProps) {
  const wheel = useRef<HTMLDivElement>(null)
  const { x, y } = wheelPoint(color)

  const moveTo = (e: PointerEvent<HTMLDivElement>) => {
    const box = wheel.current?.getBoundingClientRect()
    if (!box) return
    const radius = box.width / 2
    onChange({ ...color, ...pointToHueSaturation({ x: (e.clientX - box.left - radius) / radius, y: (e.clientY - box.top - radius) / radius }) })
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const next = nudge(color, e.key, e.shiftKey ? BIG_STEP : STEP)
    if (next === color) return
    e.preventDefault()
    onChange(next)
  }

  return (
    <div
      ref={wheel}
      role="slider"
      tabIndex={0}
      aria-label="Hue and saturation"
      aria-valuemin={0}
      aria-valuemax={360}
      aria-valuenow={Math.round(color.h)}
      aria-valuetext={`Hue ${Math.round(color.h)} degrees, saturation ${Math.round(color.s * 100)} percent`}
      className="relative aspect-square w-full cursor-crosshair touch-none rounded-full outline-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--accent) focus-visible:outline-solid"
      style={{ backgroundImage: 'var(--accent-wheel-hues)' }}
      onPointerDown={(e) => {
        e.currentTarget.setPointerCapture(e.pointerId)
        moveTo(e)
      }}
      onPointerMove={(e) => {
        if (e.currentTarget.hasPointerCapture(e.pointerId)) moveTo(e)
      }}
      onKeyDown={onKeyDown}
    >
      <span aria-hidden="true" className="pointer-events-none absolute inset-0 rounded-full" style={{ backgroundImage: 'radial-gradient(circle closest-side, var(--accent-wheel-white), transparent)' }} />
      <span aria-hidden="true" className="pointer-events-none absolute inset-0 rounded-full" style={{ backgroundColor: 'var(--accent-wheel-black)', opacity: 1 - color.v }} />
      <span
        aria-hidden="true"
        className="pointer-events-none absolute size-5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-(--accent-wheel-white)"
        style={{ left: `${50 + x * 50}%`, top: `${50 + y * 50}%`, backgroundColor: hsvToHex(color), boxShadow: '0 0 0 1px var(--accent-wheel-black)' }}
      />
    </div>
  )
}
