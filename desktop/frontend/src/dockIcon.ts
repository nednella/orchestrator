import type { ThemeName } from './types'

// themeFollower calls show with the first theme it is given and with each change of it, never twice for one theme.
export function themeFollower(show: (theme: ThemeName) => void): (theme: ThemeName) => void {
  let shown: ThemeName | null = null
  return (theme) => {
    if (theme === shown) return
    shown = theme
    show(theme)
  }
}
