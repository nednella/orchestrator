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

export function modeFor(width: number): LayoutMode {
  if (width < OVERLAY_BELOW) return 'narrow'
  if (width < COMPACT_BELOW) return 'compact'
  return width < MEDIUM_BELOW ? 'medium' : 'wide'
}

// What changes the side panel open over the thread: its trigger opens or closes it, and opening one closes the
// other. Esc, a click on the thread beside it or a move to the thread, and opening a thread close it.
export type OverlayEvent = { open: SidePanel } | { close: SidePanel } | 'escape' | 'thread-focused' | 'thread-opened'

export function nextOverlay(open: SidePanel | null, event: OverlayEvent): SidePanel | null {
  if (typeof event === 'string') return null
  if ('open' in event) return event.open
  return open === event.close ? null : open
}
