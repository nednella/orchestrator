export type SidePanel = 'sidebar' | 'right'
export type Band = 'compact' | 'medium' | 'wide'
export type LayoutMode = 'narrow' | Band

// Below this window width the side panels always open over the thread, one at a time.
export const OVERLAY_BELOW = 640
const COMPACT_BELOW = 900
const MEDIUM_BELOW = 1250

export const RAIL_REM = 3
export const SIDEBAR_DEFAULT_REM = 18
export const MIN_REM: Record<SidePanel, number> = { sidebar: 13, right: 9.5 }
export const MAX_REM = 32
// The thread keeps at least this width beside a docked panel; a panel with no room for it opens over the thread.
export const THREAD_MIN_REM = 30
// The rail's border and the line beside each side panel.
const LINES_PX = 3
const RIGHT_DEFAULT_REM = 22

export type FitPrefs = { sidebarOpen: boolean; sidebar: number; right: number | null }
export type Fitted = { widths: Record<SidePanel, number>; docked: Record<SidePanel, boolean> }

// Where each side panel goes at a window width in px, and its width in rem. A panel docks beside the thread when the
// thread keeps THREAD_MIN_REM beside it, else it opens over the thread. Each panel keeps the width it was set to,
// docked or not, but an overlay is never wider than the room under the top bar.
export function fit(windowPx: number, prefs: FitPrefs, root: number): Fitted {
  const thread = THREAD_MIN_REM * root
  const wideEnough = windowPx >= OVERLAY_BELOW
  const sidebarPx = (RAIL_REM + prefs.sidebar) * root
  const sidebarDocks = wideEnough && windowPx - sidebarPx - LINES_PX >= thread
  const left = sidebarDocks && prefs.sidebarOpen ? sidebarPx + LINES_PX : 0
  const right = prefs.right ?? RIGHT_DEFAULT_REM
  const rightDocks = wideEnough && windowPx - left - LINES_PX - right * root >= thread
  const room = (windowPx - left) / root
  return {
    widths: {
      sidebar: sidebarDocks ? prefs.sidebar : Math.min(prefs.sidebar, windowPx / root - RAIL_REM),
      right: Math.min(right, room),
    },
    docked: { sidebar: sidebarDocks, right: rightDocks },
  }
}

// The widest a docked panel can be dragged: the thread keeps THREAD_MIN_REM beside it, and the other panel, docked and
// open, keeps its place, so a drag never moves a panel over the thread. Floored to a hundredth of a rem, so that
// rounding never tips fit the other way.
export function dockedMax(panel: SidePanel, windowPx: number, prefs: FitPrefs & { rightOpen: boolean }, root: number): number {
  const { docked } = fit(windowPx, prefs, root)
  const room = windowPx - THREAD_MIN_REM * root - LINES_PX
  const sidebarBeside = docked.sidebar && prefs.sidebarOpen ? (RAIL_REM + prefs.sidebar) * root + LINES_PX : 0
  const rightBeside = docked.right && prefs.rightOpen ? (prefs.right ?? RIGHT_DEFAULT_REM) * root + LINES_PX : 0
  const max = panel === 'sidebar' ? (room - rightBeside) / root - RAIL_REM : (room - sidebarBeside) / root
  return Math.floor(max * 100) / 100
}

// Opening the docked sidebar can leave the docked right panel no room beside the thread. The owner opened both, so
// the right panel then stays open, over the thread.
export function sidebarPushesRightOver(windowPx: number, prefs: FitPrefs, root: number): boolean {
  return fit(windowPx, { ...prefs, sidebarOpen: false }, root).docked.right && !fit(windowPx, { ...prefs, sidebarOpen: true }, root).docked.right
}

export function modeFor(width: number): LayoutMode {
  if (width < OVERLAY_BELOW) return 'narrow'
  if (width < COMPACT_BELOW) return 'compact'
  return width < MEDIUM_BELOW ? 'medium' : 'wide'
}

// What changes the side panel open over the thread: its trigger opens or closes it, and opening one closes the
// other. Esc, a click on the thread beside it or a move to the thread, and opening a thread close it.
export type OverlayEvent = { open: SidePanel } | { close: SidePanel } | 'escape' | 'thread-focused' | 'thread-opened'

// An overlay the owner closes (its trigger, Esc, a click on the thread) closes its panel for good, so the panel does
// not dock open again when the window widens. Opening a thread only puts the overlay away.
export function closesPanel(event: OverlayEvent): boolean {
  return typeof event === 'string' ? event !== 'thread-opened' : 'close' in event
}

export function nextOverlay(open: SidePanel | null, event: OverlayEvent): SidePanel | null {
  if (typeof event === 'string') return null
  if ('open' in event) return event.open
  return open === event.close ? null : open
}
