export type SidePanel = 'sidebar' | 'right'

export const RAIL_REM = 3
export const SIDEBAR_DEFAULT_REM = 18
export const MIN_REM: Record<SidePanel, number> = { sidebar: 13, right: 9.5 }
export const MAX_REM = 32
const TERMINAL_MIN_PX = 280
// The rail's border and the line beside each side panel.
const LINES_PX = 3
const RIGHT_SHARE = 0.24
const RIGHT_DEFAULT_RANGE: [number, number] = [9.5, 22]

export type FitPrefs = { sidebarOpen: boolean; rightOpen: boolean; sidebar: number; right: number | null }
export type Fitted = { widths: Record<SidePanel, number>; rightForcedClosed: boolean }

const clamp = (value: number, [min, max]: [number, number]) => Math.min(Math.max(value, min), max)

// The side panels' widths in rem for a window width in px. The sidebar keeps the width it was set to at every window
// width; the right panel shrinks to leave the thread its room, and is closed when even its smallest width does not fit.
export function fit(window: number, prefs: FitPrefs, root: number): Fitted {
  const sidebar = prefs.sidebar
  const right = prefs.right ?? clamp((RIGHT_SHARE * window) / root, RIGHT_DEFAULT_RANGE)
  if (!prefs.rightOpen) return { widths: { sidebar, right }, rightForcedClosed: false }
  const left = prefs.sidebarOpen ? (RAIL_REM + sidebar) * root : 0
  const room = (window - left - LINES_PX - TERMINAL_MIN_PX) / root
  if (room < MIN_REM.right) return { widths: { sidebar, right: 0 }, rightForcedClosed: true }
  return { widths: { sidebar, right: Math.min(right, room) }, rightForcedClosed: false }
}
