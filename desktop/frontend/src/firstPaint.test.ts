import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { FIRST_PAINT_KEY, saveFirstPaint, savedFirstPaint } from './firstPaint.ts'

const data = new Map<string, string>()
Object.assign(globalThis, {
  localStorage: {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => void data.set(key, value),
    removeItem: (key: string) => void data.delete(key),
  },
})

const html = readFileSync(new URL('../index.html', import.meta.url), 'utf8')
const script = html.match(/<script>([\s\S]*?)<\/script>/)?.[1] ?? ''

function themeBeforeReact(systemDark: boolean): string | undefined {
  const root = { dataset: {} as Record<string, string> }
  new Function('document', 'matchMedia', 'localStorage', script)({ documentElement: root }, () => ({ matches: systemDark }), localStorage)
  return root.dataset.theme
}

test('a first run follows the system with the default themes', () => {
  data.clear()
  assert.deepEqual(savedFirstPaint(), { theme: 'system', lightTheme: 'light', darkTheme: 'black', lightAccent: '', darkAccent: '' })
  assert.equal(themeBeforeReact(true), 'black')
  assert.equal(themeBeforeReact(false), 'light')
})

test('the saved copy gives the settings back', () => {
  data.clear()
  saveFirstPaint({ theme: 'dark', lightTheme: 'linen', darkTheme: 'umber', lightAccent: '', darkAccent: '#ff0000' })
  assert.deepEqual(savedFirstPaint(), { theme: 'dark', lightTheme: 'linen', darkTheme: 'umber', lightAccent: '', darkAccent: '#ff0000' })
})

test('the page wears the saved theme before React loads', () => {
  data.clear()
  const save = (theme: 'system' | 'light' | 'dark') => saveFirstPaint({ theme, lightTheme: 'linen', darkTheme: 'midnight', lightAccent: '', darkAccent: '' })
  save('light')
  assert.equal(themeBeforeReact(true), 'linen')
  save('dark')
  assert.equal(themeBeforeReact(false), 'midnight')
  save('system')
  assert.equal(themeBeforeReact(true), 'midnight')
  assert.equal(themeBeforeReact(false), 'linen')
})

test('a damaged copy falls back to the system appearance', () => {
  data.clear()
  data.set(FIRST_PAINT_KEY, '{nope')
  assert.equal(themeBeforeReact(true), 'black')
  data.set(FIRST_PAINT_KEY, 'null')
  assert.equal(themeBeforeReact(false), 'light')
})
