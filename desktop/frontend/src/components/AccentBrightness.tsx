import type { CSSProperties } from 'react'
import { hsvToHex } from '../accentWheel'
import type { Hsv } from '../accentWheel'

type AccentBrightnessProps = { color: Hsv; onChange(color: Hsv): void }

// AccentBrightness is the slider under the wheel: black at its left end, the color at full brightness at its right.
export function AccentBrightness({ color, onChange }: AccentBrightnessProps) {
  const style = { '--accent-brightness-full': hsvToHex({ ...color, v: 1 }), '--accent-brightness-now': hsvToHex(color) } as CSSProperties
  return (
    <input
      type="range"
      min={0}
      max={100}
      step={1}
      aria-label="Brightness"
      value={Math.round(color.v * 100)}
      className="accent-brightness w-full"
      style={style}
      onChange={(e) => onChange({ ...color, v: Number(e.target.value) / 100 })}
    />
  )
}
