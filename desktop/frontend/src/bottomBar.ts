export type BarButtonId = 'settings' | 'pulls' | 'issues' | 'usage' | 'terminal' | 'news'

export type BarButton = { id: BarButtonId; label: string }

export const BAR_BUTTONS: readonly BarButton[] = [
  { id: 'settings', label: 'Settings' },
  { id: 'pulls', label: 'Pull requests' },
  { id: 'issues', label: 'Issues' },
  { id: 'usage', label: 'Usage' },
  { id: 'terminal', label: 'Terminal' },
  { id: 'news', label: 'News' },
]

const BUTTON = 32
const GAP = 4
const SIDE = 8
const UPDATE = 32

// barLayout splits the buttons for a bar width wide: those beside the update button, and those the More button's menu
// lists, the last ones first, when they do not all fit. More takes the room of one button.
export function barLayout(width: number): { shown: BarButton[]; more: BarButton[] } {
  const room = width - 2 * SIDE - GAP - UPDATE
  const fits = Math.max(0, Math.floor((room + GAP) / (BUTTON + GAP)))
  if (fits >= BAR_BUTTONS.length) return { shown: [...BAR_BUTTONS], more: [] }
  const shown = Math.max(0, fits - 1)
  return { shown: BAR_BUTTONS.slice(0, shown), more: BAR_BUTTONS.slice(shown) }
}
