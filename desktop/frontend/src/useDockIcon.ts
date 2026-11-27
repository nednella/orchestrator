import { useEffect, useState } from 'react'
import { api } from './api'
import { themeFollower } from './dockIcon'
import { useLayout } from './LayoutContext'

// useDockIcon keeps the dock icon on the icon of the theme the window shows.
export function useDockIcon() {
  const { theme } = useLayout()
  const [follow] = useState(() => themeFollower((shown) => void api.showDockIcon(shown)))
  useEffect(() => follow(theme), [follow, theme])
}
