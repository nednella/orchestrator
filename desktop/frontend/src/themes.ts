import type { DarkTheme, LightTheme, Settings } from './types'

export type Appearance = 'light' | 'dark'

// ThemeHalves is the theme each appearance shows: what the System, Light and Dark tiles draw and the window wears.
export type ThemeHalves = { light: LightTheme; dark: DarkTheme }

export type ThemePreview = { mode: 'light'; theme: LightTheme } | { mode: 'dark'; theme: DarkTheme }

// ThemeCardDef is one card of the theme library, with a preview for each appearance the theme has.
export type ThemeCardDef = { id: string; label: string; previews: ThemePreview[] }

export const THEME_CARDS: ThemeCardDef[] = [
  {
    id: 'default',
    label: 'Default',
    previews: [
      { mode: 'light', theme: 'light' },
      { mode: 'dark', theme: 'black' },
    ],
  },
]

const known = (mode: Appearance): string[] => THEME_CARDS.flatMap((card) => card.previews.filter((p) => p.mode === mode).map((p) => p.theme))

// themeHalves is the saved themes, with a name this build has no theme for, such as a removed one, shown as the default.
export function themeHalves({ lightTheme, darkTheme }: Pick<Settings, 'lightTheme' | 'darkTheme'>): ThemeHalves {
  return {
    light: known('light').includes(lightTheme) ? lightTheme : 'light',
    dark: known('dark').includes(darkTheme) ? darkTheme : 'black',
  }
}

// pickedModes is the appearances that show the card's theme.
export function pickedModes(card: ThemeCardDef, halves: ThemeHalves): Appearance[] {
  return card.previews.filter((p) => halves[p.mode] === p.theme).map((p) => p.mode)
}

// withCard is the halves once the card is used for mode, or for every appearance it has when mode is not given.
export function withCard(card: ThemeCardDef, halves: ThemeHalves, mode?: Appearance): ThemeHalves {
  const next = { ...halves }
  for (const preview of card.previews) {
    if (mode && preview.mode !== mode) continue
    if (preview.mode === 'light') next.light = preview.theme
    else next.dark = preview.theme
  }
  return next
}

export const modeTip = (mode: Appearance) => (mode === 'light' ? 'Use for light mode only' : 'Use for dark mode only')

export const cardTip = (card: ThemeCardDef) => (card.previews.length > 1 ? 'Use for both light and dark' : modeTip(card.previews[0].mode))
