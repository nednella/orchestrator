import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import type { DarkTheme, LightTheme } from './types.ts'
import { cardTip, pickedModes, THEME_CARDS, themeHalves, withCard } from './themes.ts'

const card = (id: string) => THEME_CARDS.find((c) => c.id === id)!
const standard = { light: 'light', dark: 'black' } as const

test('the default card takes both halves, or the one picked by its circle', () => {
  assert.deepEqual(withCard(card('default'), standard), standard)
  assert.deepEqual(withCard(card('default'), standard, 'dark'), standard)
})

test('a card rings the appearances that show it', () => {
  assert.deepEqual(pickedModes(card('default'), standard), ['light', 'dark'])
})

test('a saved theme this build does not have shows the default', () => {
  assert.deepEqual(themeHalves({ lightTheme: 'sepia' as LightTheme, darkTheme: 'violet' as DarkTheme }), standard)
})

test('a card says which appearances it is for', () => {
  assert.equal(cardTip(card('default')), 'Use for both light and dark')
})

test('every theme in the library has its colours in tokens.css', () => {
  const css = readFileSync(new URL('./tokens.css', import.meta.url), 'utf8')
  for (const { previews } of THEME_CARDS) for (const { theme } of previews) assert.equal(css.includes(`[data-preview='${theme}']`), true, theme)
})
