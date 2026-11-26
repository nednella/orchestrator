export type SidePanel = 'sidebar' | 'right'
export type Band = 'compact' | 'medium' | 'wide'
export type LayoutMode = 'narrow' | Band

// Below this window width the side panels always open over the thread, one at a time.
export const OVERLAY_BELOW = 640
const COMPACT_BELOW = 900
const MEDIUM_BELOW = 1250

export const RAIL_REM = 3
export const SIDEBAR_DEFAULT_REM = 18
export const SIDEBAR_MIN_REM = 13
export const SIDEBAR_MAX_REM = 32
// The thread keeps at least this width beside the docked sidebar; a sidebar with no room for it opens over the thread.
export const THREAD_MIN_REM = 30
// The rail's border and the line beside each side panel.
const LINES_PX = 3

// sidebarFit is where the sidebar goes at a window width in px, and its width in rem. It docks beside the thread when
// the thread keeps THREAD_MIN_REM beside it, else it opens over the thread. It keeps the width it was set to, docked or
// not, but an overlay is never wider than the window.
export function sidebarFit(windowPx: number, sidebar: number, root: number): { width: number; docked: boolean } {
  const docked = windowPx >= OVERLAY_BELOW && windowPx - (RAIL_REM + sidebar) * root - LINES_PX >= THREAD_MIN_REM * root
  return { width: docked ? sidebar : Math.min(sidebar, windowPx / root - RAIL_REM), docked }
}

// sidebarMax is the widest the docked sidebar can be dragged: the thread keeps THREAD_MIN_REM beside it, and beside
// the docked right panel, open, at rightPx, its least width, so a drag never moves the sidebar over the thread.
// Floored to a hundredth of a rem, so that rounding never tips sidebarFit the other way.
export function sidebarMax(windowPx: number, rightPx: number, root: number): number {
  const max = (windowPx - THREAD_MIN_REM * root - LINES_PX - rightPx) / root - RAIL_REM
  return Math.floor(max * 100) / 100
}

// sidebarBesidePx is the room in px the sidebar takes from the row the chat and the right panel share: none while it is
// closed or over the thread.
export function sidebarBesidePx(sidebar: { width: number; docked: boolean }, open: boolean, root: number): number {
  return sidebar.docked && open ? (RAIL_REM + sidebar.width) * root + 1 : 0
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
