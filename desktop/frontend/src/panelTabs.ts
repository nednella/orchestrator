import type { FocusTarget } from './OrchestratorContext'

export type PanelViewId = 'notes' | 'files' | 'issues' | 'pulls' | 'browser' | 'media' | 'news' | 'terminal'

// The letter that opens each view from the panel's list.
export const PANEL_KEYS: Record<PanelViewId, string> = { notes: 'N', files: 'F', issues: 'I', pulls: 'P', browser: 'B', media: 'M', news: 'W', terminal: 'T' }

// The views the panel lists, in its order.
const PANEL_ORDER: PanelViewId[] = ['notes', 'files', 'issues', 'pulls', 'browser', 'media', 'news', 'terminal']

// What a row of the panel's list does: open a view, or start a side chat of the open thread.
export type PanelRowTarget = PanelViewId | 'sideChat'
export const SIDE_CHAT_KEY = 'S'

// A tab of the right panel: a view, or a side chat of the thread by its id.
export type SideChatTab = `side:${string}`
export type PanelTabId = PanelViewId | SideChatTab

export const sideChatTab = (id: string): SideChatTab => `side:${id}`

// sideChatOf is the id of the side chat the tab shows, null for a view's tab.
export const sideChatOf = (tab: PanelTabId | null): string | null => (tab?.startsWith('side:') ? tab.slice('side:'.length) : null)

// viewOfTab is the view the tab shows, null for a side chat's tab.
export const viewOfTab = (tab: PanelTabId | null): PanelViewId | null => (tab === null || sideChatOf(tab) !== null ? null : (tab as PanelViewId))

// What the right panel shows for one thread, or for Home: the tabs open, the one shown (null for the list of views),
// and whether it takes the chat's room too. The Terminal view has no tab of its own here: its tabs are the project's
// terminal groups, the same in every thread, so it is shown without being in tabs.
export type PanelState = { tabs: PanelTabId[]; active: PanelTabId | null; expanded: boolean }

export const EMPTY_PANEL: PanelState = { tabs: [], active: null, expanded: false }

export function openTab(st: PanelState, view: PanelTabId): PanelState {
  if (view === 'terminal') return { ...st, active: view }
  return { ...st, tabs: st.tabs.includes(view) ? st.tabs : [...st.tabs, view], active: view }
}

// closeTab closes the view's tab. A closed active tab hands over to its left neighbour, else to the tab after it; with
// no tab left the list shows, at the panel's own size.
export function closeTab(st: PanelState, view: PanelTabId): PanelState {
  const at = st.tabs.indexOf(view)
  if (at < 0) return st
  const tabs = st.tabs.filter((tab) => tab !== view)
  if (tabs.length === 0 && st.active !== 'terminal') return EMPTY_PANEL
  return { ...st, tabs, active: st.active === view ? tabs[Math.max(0, at - 1)] : st.active }
}

// stepTab shows the tab delta steps from the one shown, round the ends; from the list, the first or the last. The
// terminal tabs come after the views', so from them the step goes to the first or the last view.
export function stepTab(st: PanelState, delta: -1 | 1): PanelState {
  if (st.tabs.length === 0) return st
  const at = st.active === null ? (delta > 0 ? -1 : 0) : st.active === 'terminal' ? (delta > 0 ? st.tabs.length - 1 : st.tabs.length) : st.tabs.indexOf(st.active)
  return { ...st, active: st.tabs[(at + delta + st.tabs.length) % st.tabs.length] }
}

// What the chat column holds, which the expanded panel hides: the focus targets in it.
const CHAT_COLUMN: FocusTarget[] = ['thread', 'composer']

// panelOnFocus gives the chat column its room back when the focus moves into it: the expanded panel goes back to its
// own size.
export function panelOnFocus(st: PanelState, target: FocusTarget): PanelState {
  return st.expanded && CHAT_COLUMN.includes(target) ? { ...st, expanded: false } : st
}

// targetForKey is what the letter key does from the panel's list, null for a letter that does nothing.
export function targetForKey(key: string): PanelRowTarget | null {
  if (key.toUpperCase() === SIDE_CHAT_KEY) return 'sideChat'
  return PANEL_ORDER.find((view) => PANEL_KEYS[view] === key.toUpperCase()) ?? null
}

// panelStateOf is a saved panel as the panel can show it: a view it no longer lists drops out, and Media is what Evidence
// was saved as.
export function panelStateOf(saved: unknown): PanelState {
  if (typeof saved !== 'object' || saved === null) return EMPTY_PANEL
  const { tabs, active, expanded } = saved as Record<string, unknown>
  const known = (tab: unknown): tab is PanelTabId => PANEL_ORDER.includes(tab as PanelViewId) || (typeof tab === 'string' && sideChatOf(tab as PanelTabId) !== null && tab.length > 'side:'.length)
  const renamed = (view: unknown) => (view === 'evidence' ? 'media' : view)
  const kept: PanelTabId[] = Array.isArray(tabs) ? [...new Set(tabs.map(renamed).filter(known))].filter((view) => view !== 'terminal') : []
  const shown = renamed(active)
  return { tabs: kept, active: known(shown) && (kept.includes(shown) || shown === 'terminal') ? shown : null, expanded: expanded === true }
}

// The least widths, in px at the default text size, of the chat and of the panel beside it.
const CHAT_MIN = 360
const PANEL_MIN = 360

// At this window width or less the panel opens over the chat as a sheet, and so it does where the row has no room for
// the chat and the panel at their least.
const SHEET_AT = 980
const SHEET_NARROW_AT = 760
const ROW_MIN = CHAT_MIN + PANEL_MIN

// panelWidth is how wide the panel beside the chat can be, in px at the default text size, for a window viewport wide
// and a row, the room the chat and the panel share: the chat keeps CHAT_MIN.
export function panelWidth(viewport: number, row: number): { min: number; max: number; def: number } {
  return { min: PANEL_MIN, max: Math.floor(Math.min(0.7 * viewport, row - CHAT_MIN)), def: 540 }
}

// panelFit says whether the panel sits beside the chat and how wide it is, in px at the default text size, for its
// saved width. A sheet has a width of its own.
export function panelFit(viewport: number, row: number, saved: number | null): { docked: boolean; width: number } {
  if (viewport <= SHEET_NARROW_AT) return { docked: false, width: Math.floor(Math.min(0.88 * viewport, 384)) }
  if (viewport <= SHEET_AT || row < ROW_MIN) return { docked: false, width: Math.max(320, Math.floor(Math.min(0.42 * viewport, 448))) }
  const { min, max, def } = panelWidth(viewport, row)
  return { docked: true, width: Math.max(min, Math.min(saved ?? def, max)) }
}
