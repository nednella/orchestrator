import { readStored, writeStored } from './storage.ts'
import type { Settings } from './types'

// The key the inline script in index.html reads before the bundle loads.
export const FIRST_PAINT_KEY = 'orchestrator.firstPaint'

export type FirstPaint = Pick<Settings, 'theme' | 'lightTheme' | 'darkTheme' | 'lightAccent' | 'darkAccent'>

const DEFAULT: FirstPaint = { theme: 'system', lightTheme: 'light', darkTheme: 'black', lightAccent: '', darkAccent: '' }

// savedFirstPaint is what the last run showed, so the first render wears the owner's theme before the settings load.
export function savedFirstPaint(): FirstPaint {
  return { ...DEFAULT, ...readStored<Partial<FirstPaint>>(FIRST_PAINT_KEY, {}) }
}

export function saveFirstPaint(settings: FirstPaint) {
  const { theme, lightTheme, darkTheme, lightAccent, darkAccent } = settings
  writeStored(FIRST_PAINT_KEY, { theme, lightTheme, darkTheme, lightAccent, darkAccent })
}
