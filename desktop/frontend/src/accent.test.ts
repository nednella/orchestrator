import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { accentChoice, ACCENT_PRESETS, applyAccent, contrast, presetToken, textOnAccent } from './accent.ts'

const css = readFileSync(new URL('./tokens.css', import.meta.url), 'utf8')
const token = (name: string) => css.match(new RegExp(`${name}:\\s*(#[0-9a-f]{6});`))?.[1]

test('the text on an accent is black or white, whichever has the higher contrast', () => {
  assert.equal(textOnAccent('#ffffff'), '--text-on-light-accent')
  assert.equal(textOnAccent('#000000'), '--text-on-dark-accent')
  assert.equal(textOnAccent('#f47db9'), '--text-on-light-accent')
  assert.equal(textOnAccent('#145ec1'), '--text-on-dark-accent')
  // A mid grey: 4.48:1 with white, 4.69:1 with black.
  assert.equal(textOnAccent('#777777'), '--text-on-light-accent')
  assert.equal(textOnAccent('#757575'), '--text-on-dark-accent')
})

test('contrast follows the WCAG ratio', () => {
  assert.equal(contrast('#000000', '#ffffff'), 21)
  assert.equal(contrast('#ffffff', '#000000'), 21)
  assert.equal(contrast('#777777', '#777777'), 1)
})

function fakeStyle() {
  const props = new Map<string, string>()
  return {
    props,
    setProperty: (name: string, value: string | null) => void props.set(name, value ?? ''),
    removeProperty: (name: string) => {
      const was = props.get(name) ?? ''
      props.delete(name)
      return was
    },
  }
}

test('a picked accent overrides the theme on the root, and none gives the theme its own back', () => {
  const style = fakeStyle()
  applyAccent(style, '#f47db9')
  assert.deepEqual(Object.fromEntries(style.props), { '--accent': '#f47db9', '--text-on-accent': 'var(--text-on-light-accent)' })
  applyAccent(style, '#be222a')
  assert.deepEqual(Object.fromEntries(style.props), { '--accent': '#be222a', '--text-on-accent': 'var(--text-on-dark-accent)' })
  applyAccent(style, '')
  assert.deepEqual(Object.fromEntries(style.props), {})
  applyAccent(style, '#f47db9')
  applyAccent(style, 'pink')
  assert.deepEqual(Object.fromEntries(style.props), {}, 'a value that is not #rrggbb is no accent')
})

test('the settings pick the theme, a preset or a custom color', () => {
  const color = (preset: string) => token(presetToken('dark', preset as 'pink'))!
  assert.equal(accentChoice('', color), 'theme')
  assert.equal(accentChoice('#f47db9', color), 'pink')
  assert.equal(accentChoice('#F47DB9', color), 'pink')
  assert.equal(accentChoice('#123456', color), 'custom')
})

test('each preset reads on its appearance: dark ones on Black, light ones on Light', () => {
  for (const { id } of ACCENT_PRESETS) {
    const dark = token(presetToken('dark', id)) ?? ''
    const light = token(presetToken('light', id)) ?? ''
    assert.equal(dark !== '' && light !== '', true, `${id} has a #rrggbb token for each appearance`)
    for (const bg of ['#000000', '#121212']) assert.equal(contrast(dark, bg) >= 4.5, true, `dark ${id} on ${bg}: ${contrast(dark, bg).toFixed(2)}`)
    for (const bg of ['#fcfcfc', '#ffffff', '#eceef3']) assert.equal(contrast(light, bg) >= 4.5, true, `light ${id} on ${bg}: ${contrast(light, bg).toFixed(2)}`)
  }
})

// toHex reads a #rrggbb or an oklch(L C h) token as #rrggbb.
function toHex(value: string): string {
  const oklch = value.match(/^oklch\(([\d.]+) ([\d.]+) ([\d.]+)\)$/)
  if (!oklch) return value.length === 4 ? '#' + [...value.slice(1)].map((c) => c + c).join('') : value
  const [L, C] = [Number(oklch[1]), Number(oklch[2])]
  const a = C * Math.cos((Number(oklch[3]) * Math.PI) / 180)
  const b = C * Math.sin((Number(oklch[3]) * Math.PI) / 180)
  const [l, m, s] = [L + 0.3963377774 * a + 0.2158037573 * b, L - 0.1055613458 * a - 0.0638541728 * b, L - 0.0894841775 * a - 1.291485548 * b].map((x) => x ** 3)
  const linear = [4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s, -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s, -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s]
  const channel = (x: number) => {
    const c = Math.min(1, Math.max(0, x))
    return Math.round((c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055) * 255)
  }
  return '#' + linear.map((x) => channel(x).toString(16).padStart(2, '0')).join('')
}

test('text, soft and dim text read at 4.5:1 on every surface of every theme half', () => {
  const halves = [...css.matchAll(/(?::root\[data-theme='([a-z-]+)'\]|:root),[^{]*\{([\s\S]*?)\n\}/g)].filter((m) => m[2].includes('--bg-app:'))
  assert.equal(halves.length, 10)
  const own = (body: string, name: string) => body.match(new RegExp(`${name}:\\s*((?:#[0-9a-f]{3,6}|oklch\\([^)]*\\)));`))?.[1]
  for (const [, theme = 'black', body] of halves) {
    const texts = ['--text', '--text-soft', '--text-dim'].map((name) => [name, toHex(own(body, name) ?? '')] as const)
    for (const bgName of ['--bg-app', '--bg-surface', '--bg-raised', '--bg-hover']) {
      const bg = toHex(own(body, bgName) ?? '')
      for (const [name, fg] of texts) assert.equal(contrast(fg, bg) >= 4.5, true, `${theme} ${name} on ${bgName}: ${contrast(fg, bg).toFixed(2)}`)
    }
    const [text, soft, dim] = texts.map(([, fg]) => fg)
    const app = toHex(own(body, '--bg-app') ?? '')
    assert.equal(contrast(text, app) > contrast(soft, app) && contrast(soft, app) > contrast(dim, app), true, `${theme}: dim is softer than soft, soft than text`)
  }
})
