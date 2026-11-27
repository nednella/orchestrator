import type { Appearance } from './themes'

export const ACCENT_PRESETS = [
  { id: 'blue', label: 'Blue' },
  { id: 'violet', label: 'Violet' },
  { id: 'pink', label: 'Pink' },
  { id: 'red', label: 'Red' },
  { id: 'orange', label: 'Orange' },
  { id: 'green', label: 'Green' },
] as const

export type AccentPreset = (typeof ACCENT_PRESETS)[number]['id']

// The token in tokens.css that holds a preset's color for an appearance.
export const presetToken = (mode: Appearance, preset: AccentPreset) => `--accent-preset-${mode}-${preset}`

const HEX = /^#[0-9a-f]{6}$/i

// luminance is a #rrggbb color's relative luminance, as WCAG defines it.
function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

// contrast is the WCAG contrast ratio of two #rrggbb colors, from 1 to 21.
export function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

// textOnAccent is the token of the text color that reads better on the accent: black or white.
export function textOnAccent(accent: string): '--text-on-light-accent' | '--text-on-dark-accent' {
  return contrast(accent, '#000000') >= contrast(accent, '#ffffff') ? '--text-on-light-accent' : '--text-on-dark-accent'
}

type RootStyle = Pick<CSSStyleDeclaration, 'setProperty' | 'removeProperty'>

// applyAccent sets the accent the owner picked on the root, and the text that goes on it; with none, the theme's own
// come back.
export function applyAccent(style: RootStyle, accent: string) {
  if (!HEX.test(accent)) {
    style.removeProperty('--accent')
    style.removeProperty('--text-on-accent')
    return
  }
  style.setProperty('--accent', accent)
  style.setProperty('--text-on-accent', `var(${textOnAccent(accent)})`)
}

// accentChoice is what the settings show picked for an accent: the theme's own, the preset whose color it is, or a
// custom color. presetColor reads a preset's color.
export function accentChoice(accent: string, presetColor: (preset: AccentPreset) => string): 'theme' | AccentPreset | 'custom' {
  if (accent === '') return 'theme'
  const preset = ACCENT_PRESETS.find((p) => presetColor(p.id).toLowerCase() === accent.toLowerCase())
  return preset ? preset.id : 'custom'
}
