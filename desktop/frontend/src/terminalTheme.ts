import type { ITheme } from '@xterm/xterm'

const ansi = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white'] as const

// xterm parses only hex and rgb colours, so a value the CSS writes another way (oklch) goes through a canvas pixel.
const probe = document.createElement('canvas').getContext('2d', { willReadFrequently: true })

function toRgb(color: string): string {
  if (!probe || /^(#|rgb)/.test(color)) return color
  probe.clearRect(0, 0, 1, 1)
  probe.fillStyle = color
  probe.fillRect(0, 0, 1, 1)
  const [r, g, b] = probe.getImageData(0, 0, 1, 1).data
  return `rgb(${r}, ${g}, ${b})`
}

// terminalThemeFrom reads the terminal colours from the CSS variables in effect at el, so each named theme's terminal
// matches it: the page's own background and text, and the ANSI colours of a light or a dark theme.
export function terminalThemeFrom(el: Element): ITheme {
  const style = getComputedStyle(el)
  const get = (name: string) => toRgb(style.getPropertyValue(name).trim())
  const background = get('--bg-app')
  const theme: ITheme = {
    background,
    foreground: get('--text'),
    cursor: get('--term-cursor'),
    cursorAccent: background,
    selectionBackground: get('--term-selection'),
  }
  for (const name of ansi) {
    theme[name] = get(`--term-${name}`)
    theme[`bright${name[0].toUpperCase()}${name.slice(1)}` as 'brightBlack'] = get(`--term-bright-${name}`)
  }
  return theme
}

// The Nerd Font after the mono stack, for the glyphs shell prompts draw that the others lack.
export const TERMINAL_FONT = `"SF Mono", SFMono-Regular, Menlo, Consolas, "Liberation Mono", "JetBrainsMono Nerd Font Mono", monospace`
