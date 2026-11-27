// Hsv is a color as hue in degrees (0 up to 360), saturation and value (0 to 1).
export type Hsv = { h: number; s: number; v: number }

// WheelPoint is a place on the wheel in units of its radius from its centre: x to the right, y down, so the wheel's
// edge is the circle of radius 1.
export type WheelPoint = { x: number; y: number }

const HEX = /^#[0-9a-f]{6}$/

export const clamp = (n: number, min: number, max: number) => Math.min(max, Math.max(min, n))

// normalizeHex is the #rrggbb a typed color stands for, in lower case with or without its #, or null if it is not one.
export function normalizeHex(text: string): string | null {
  const hex = `#${text.trim().replace(/^#/, '')}`.toLowerCase()
  return HEX.test(hex) ? hex : null
}

// hsvToHex is the #rrggbb of a color.
export function hsvToHex({ h, s, v }: Hsv): string {
  const f = (n: number) => {
    const k = (n + h / 60) % 6
    return v - v * s * Math.max(0, Math.min(k, 4 - k, 1))
  }
  return `#${[f(5), f(3), f(1)].map((c) => Math.round(c * 255).toString(16).padStart(2, '0')).join('')}`
}

// hexToHsv is the color a #rrggbb stands for; a grey has hue 0.
export function hexToHsv(hex: string): Hsv {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
  const max = Math.max(r, g, b)
  const delta = max - Math.min(r, g, b)
  let h = 0
  if (delta > 0) {
    if (max === r) h = ((g - b) / delta + 6) % 6
    else if (max === g) h = (b - r) / delta + 2
    else h = (r - g) / delta + 4
  }
  return { h: h * 60, s: max === 0 ? 0 : delta / max, v: max }
}

// wheelPoint is where a color's hue and saturation sit on the wheel: hue runs clockwise from the top, saturation from
// the centre out.
export function wheelPoint({ h, s }: Hsv): WheelPoint {
  const angle = (h * Math.PI) / 180
  return { x: s * Math.sin(angle), y: -s * Math.cos(angle) }
}

// pointToHueSaturation is the hue and saturation at a place on the wheel; beyond the edge it is the edge's.
export function pointToHueSaturation({ x, y }: WheelPoint): Pick<Hsv, 'h' | 's'> {
  const s = Math.min(1, Math.hypot(x, y))
  if (s === 0) return { h: 0, s }
  return { h: (((Math.atan2(x, -y) * 180) / Math.PI) + 360) % 360, s }
}

const ARROWS: Record<string, WheelPoint> = {
  ArrowUp: { x: 0, y: -1 },
  ArrowDown: { x: 0, y: 1 },
  ArrowLeft: { x: -1, y: 0 },
  ArrowRight: { x: 1, y: 0 },
}

// nudge moves the handle one step across the wheel for an arrow key, or returns the color as it is for any other key.
export function nudge(color: Hsv, key: string, step: number): Hsv {
  const arrow = ARROWS[key]
  if (!arrow) return color
  const at = wheelPoint(color)
  return { ...color, ...pointToHueSaturation({ x: at.x + arrow.x * step, y: at.y + arrow.y * step }) }
}
