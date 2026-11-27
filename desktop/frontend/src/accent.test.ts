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
    for (const bg of ['#f4f5f8', '#ffffff', '#eceef3']) assert.equal(contrast(light, bg) >= 4.5, true, `light ${id} on ${bg}: ${contrast(light, bg).toFixed(2)}`)
  }
})
