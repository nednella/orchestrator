import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import type { DarkTheme, LightTheme } from './types.ts'
import { cardTip, pickedModes, THEME_CARDS, themeHalves, withCard } from './themes.ts'

const card = (id: string) => THEME_CARDS.find((c) => c.id === id)!
const standard = { light: 'light', dark: 'black' } as const

test('a theme with one appearance takes only that half', () => {
  assert.deepEqual(withCard(card('midnight'), standard), { light: 'light', dark: 'midnight' })
  assert.deepEqual(withCard(card('linen'), { light: 'light', dark: 'umber' }), { light: 'linen', dark: 'umber' })
})

test('the default card takes both halves back, or the one picked by its circle', () => {
  const mixed = { light: 'linen', dark: 'umber' } as const
  assert.deepEqual(withCard(card('default'), mixed), standard)
  assert.deepEqual(withCard(card('default'), mixed, 'dark'), { light: 'linen', dark: 'black' })
  assert.deepEqual(withCard(card('default'), mixed, 'light'), { light: 'light', dark: 'umber' })
})

test('a card rings the appearances that show it', () => {
  const halves = { light: 'light', dark: 'midnight' } as const
  assert.deepEqual(pickedModes(card('default'), halves), ['light'])
  assert.deepEqual(pickedModes(card('midnight'), halves), ['dark'])
  assert.deepEqual(pickedModes(card('umber'), halves), [])
})

test('a saved theme this build does not have shows the default', () => {
  assert.deepEqual(themeHalves({ lightTheme: 'sepia' as LightTheme, darkTheme: 'violet' as DarkTheme }), standard)
  assert.deepEqual(themeHalves({ lightTheme: 'linen', darkTheme: 'umber' }), { light: 'linen', dark: 'umber' })
})

test('a card says which appearances it is for', () => {
  assert.equal(cardTip(card('default')), 'Use for both light and dark')
  assert.equal(cardTip(card('midnight')), 'Use for dark mode only')
  assert.equal(cardTip(card('linen')), 'Use for light mode only')
})

test('every theme in the library has its colours in tokens.css', () => {
  const css = readFileSync(new URL('./tokens.css', import.meta.url), 'utf8')
  for (const { previews } of THEME_CARDS) for (const { theme } of previews) assert.equal(css.includes(`[data-preview='${theme}']`), true, theme)
})
