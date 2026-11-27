import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { applyAccent } from './accent'
import { saveFirstPaint } from './firstPaint'
import { useOrchestrator } from './OrchestratorContext'
import type { FocusTarget } from './OrchestratorContext'
import { closesPanel, modeFor, nextOverlay, SIDEBAR_DEFAULT_REM, SIDEBAR_MAX_REM, SIDEBAR_MIN_REM, sidebarBesidePx, sidebarFit, sidebarMax } from './panelLayout'
import type { Band, LayoutMode, OverlayEvent, SidePanel } from './panelLayout'
import { updatePanel, usePanel } from './panelStore'
import { closeTab, openTab, panelFit, panelOnFocus, panelShows, panelWidth, stepTab } from './panelTabs'
import type { PanelState, PanelTabId } from './panelTabs'
import { readStored, writeStored } from './storage'
import { snapScale, zoomedScale } from './textScale'
import { themeHalves } from './themes'
import type { Appearance } from './themes'
import type { ThemeName } from './types'

const DARK_QUERY = '(prefers-color-scheme: dark)'
export type Panel = 'sidebar' | 'centre' | 'right'
export { RAIL_REM, type SidePanel } from './panelLayout'
export type TogglePanel = 'sidebar' | 'right'

// The element that holds each panel and the focus target that moves the focus into it.
const PANEL_SELECTOR: Record<TogglePanel, string> = { sidebar: '[data-sidebar]', right: '[data-panel="right"]' }
const PANEL_FOCUS: Record<TogglePanel, FocusTarget> = { sidebar: 'sidebar', right: 'right' }

const LAYOUT_KEY = 'orchestrator.layout.v3'
const SIDEBAR_WIDTH_KEY = 'orchestrator.sidebarWidth'
type BandPrefs = { sidebarOpen: boolean; rightOpen: boolean; right: number | null }
type AllPrefs = Record<Band, BandPrefs>

const fresh = (): BandPrefs => ({ sidebarOpen: true, rightOpen: false, right: null })

export type Layout = {
  mode: LayoutMode
  width: number
  scale: number
  theme: ThemeName
  sidebarOpen: boolean
  rightOpen: boolean
  // Which side panels sit beside the thread; the others open over it.
  docked: Record<SidePanel, boolean>
  // The side panel open over the thread.
  overlay: SidePanel | null
  widths: Record<SidePanel, number>
  // The open thread's right panel, or Home's; expanded only while the panel shows beside the chat and takes its room.
  panel: PanelState
  panelExpanded: boolean
  zoom(step: -1 | 0 | 1): void
  closeOverlay(why: 'escape' | 'thread-focused'): void
  setWidth(panel: SidePanel, rem: number, commit: boolean): void
  resetWidth(panel: SidePanel): void
  setRightOpen(open: boolean): void
  toggleRight(): void
  togglePanel(panel: TogglePanel): void
  // showPanelView opens the right panel on the view or side chat, in the panel of thread, else of the open thread;
  // expanded sets the panel's size too, except on a sheet, which has one size.
  showPanelView(view: PanelTabId, opts?: { expanded?: boolean; thread?: string }): void
  // togglePanelView closes the panel when it shows the view, as its own close button does, else shows the view.
  togglePanelView(view: PanelTabId, opts?: { expanded?: boolean }): void
  togglePanelExpanded(): void
  showPanelList(): void
  closePanelTab(view: PanelTabId): void
  stepPanelTab(delta: -1 | 1): void
  setSidebarOpen(open: boolean): void
  showHome(): void
  focusChat(): void
  focusPanel(panel: Panel): void
}

const LayoutContext = createContext<Layout | null>(null)

export function useLayout(): Layout {
  const value = useContext(LayoutContext)
  if (!value) throw new Error('useLayout must be used inside LayoutProvider')
  return value
}

function useWindowWidth(): number {
  const [width, setWidth] = useState(window.innerWidth)
  useEffect(() => {
    const update = () => setWidth(window.innerWidth)
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [])
  return width
}

type LayoutProviderProps = { children: ReactNode }

function useSystemAppearance(): Appearance {
  const [appearance, setAppearance] = useState<Appearance>(() => (matchMedia(DARK_QUERY).matches ? 'dark' : 'light'))
  useEffect(() => {
    const query = matchMedia(DARK_QUERY)
    const onChange = (e: MediaQueryListEvent) => setAppearance(e.matches ? 'dark' : 'light')
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
  }, [])
  return appearance
}

export function LayoutProvider({ children }: LayoutProviderProps) {
  const { selectedId, project, settings, setTextScale, showHome: selectHome, focus, onFocus, report, pushToast, dismissToast } = useOrchestrator('selectedId', 'project', 'settings', 'setTextScale', 'showHome', 'focus', 'onFocus', 'report', 'pushToast', 'dismissToast')
  const width = useWindowWidth()
  const mode = modeFor(width)
  const band: Band = mode === 'narrow' ? 'compact' : mode
  const scale = snapScale(settings.textScale)
  const systemAppearance = useSystemAppearance()
  const appearance = settings.theme === 'system' ? systemAppearance : settings.theme
  const theme = themeHalves(settings)[appearance]
  const accent = appearance === 'dark' ? settings.darkAccent : settings.lightAccent
  const [stored, setStored] = useState(() => readStored<Partial<AllPrefs>>(LAYOUT_KEY, {}))
  // Sidebar widths were saved per band before: until a width is saved on its own, the band the window opens in gives
  // the width the owner set. A saved null is a reset to the default.
  const [sidebarRem, setSidebarRem] = useState(() => {
    const saved = readStored<number | null | undefined>(SIDEBAR_WIDTH_KEY, undefined)
    return saved !== undefined ? saved : (readStored<Partial<Record<Band, { sidebar?: number | null }>>>(LAYOUT_KEY, {})[band]?.sidebar ?? null)
  })
  const [openOverlay, setOpenOverlay] = useState<SidePanel | null>(null)
  const zoomToast = useRef<number | null>(null)
  const focusBefore = useRef<Partial<Record<TogglePanel, Element>>>({})

  useLayoutEffect(() => {
    document.documentElement.style.setProperty('--ui-scale', String(scale))
  }, [scale])

  useLayoutEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])

  useEffect(() => {
    saveFirstPaint(settings)
  }, [settings.theme, settings.lightTheme, settings.darkTheme, settings.lightAccent, settings.darkAccent])

  useLayoutEffect(() => {
    applyAccent(document.documentElement.style, accent)
  }, [accent])

  const prefs: BandPrefs = { ...fresh(), ...stored[band] }
  const root = 16 * scale
  // The right panel's rules are in px at the default text size, as the window would be at that size.
  const sidebar = sidebarFit(width, Math.max(sidebarRem ?? SIDEBAR_DEFAULT_REM, SIDEBAR_MIN_REM), root)
  const viewport = width / scale
  const rowBeside = (sidebarOpen: boolean) => (width - sidebarBesidePx(sidebar, sidebarOpen, root)) / scale - 1
  const row = rowBeside(prefs.sidebarOpen)
  const rowWithSidebar = rowBeside(true)
  const right = panelFit(viewport, row, prefs.right === null ? null : prefs.right * 16)
  const docked = { sidebar: sidebar.docked, right: right.docked }
  const overlay = openOverlay && !docked[openOverlay] ? openOverlay : null
  const sidebarOpen = docked.sidebar ? prefs.sidebarOpen : overlay === 'sidebar'
  const rightOpen = docked.right ? prefs.rightOpen : overlay === 'right'
  const panelKey = selectedId ?? ''
  const panel = usePanel(panelKey)
  const panelExpanded = panel.expanded && docked.right && rightOpen

  const updatePrefs = useCallback(
    (patch: Partial<BandPrefs>) =>
      setStored((all) => {
        const next = { ...all, [band]: { ...fresh(), ...all[band], ...patch } }
        writeStored(LAYOUT_KEY, next)
        return next
      }),
    [band],
  )

  const moveOverlay = useCallback((event: OverlayEvent) => setOpenOverlay((open) => nextOverlay(open, event)), [])

  useEffect(() => {
    if (openOverlay && docked[openOverlay]) setOpenOverlay(null)
  }, [openOverlay, docked.sidebar, docked.right])

  // A docked panel that held the focus and leaves the side of the thread (the window got narrower, or the sidebar
  // pushed the right panel over the thread) leaves the focus nowhere: it goes into the panel's overlay, or to the
  // thread when the panel is closed.
  const dockedBefore = useRef(docked)
  useEffect(() => {
    const before = dockedBefore.current
    dockedBefore.current = docked
    const left = (['sidebar', 'right'] as const).find((panel) => before[panel] && !docked[panel])
    if (!left || (document.activeElement && document.activeElement !== document.body)) return
    focus(overlay === left ? PANEL_FOCUS[left] : 'thread')
  }, [docked.sidebar, docked.right])

  const previousSelected = useRef(selectedId)
  useEffect(() => {
    if (previousSelected.current === selectedId) return
    previousSelected.current = selectedId
    moveOverlay('thread-opened')
  }, [selectedId, moveOverlay])

  const setSidebarOpen = useCallback(
    (open: boolean) => {
      if (!docked.sidebar) return moveOverlay(open ? { open: 'sidebar' } : { close: 'sidebar' })
      // A sidebar that leaves the chat and the panel no room beside each other sends the open panel over the chat.
      if (open && docked.right && prefs.rightOpen && !panelFit(viewport, rowWithSidebar, null).docked) moveOverlay({ open: 'right' })
      updatePrefs({ sidebarOpen: open })
    },
    [docked.sidebar, docked.right, prefs.rightOpen, viewport, rowWithSidebar, moveOverlay, updatePrefs],
  )

  // A panel closed another way (its close button, the other overlay) forgets where the focus was.
  useEffect(() => {
    const shown: Record<TogglePanel, boolean> = { sidebar: sidebarOpen, right: rightOpen }
    for (const panel of Object.keys(shown) as TogglePanel[]) if (!shown[panel]) delete focusBefore.current[panel]
  }, [sidebarOpen, rightOpen])

  // A closing panel that holds the focus gives it back to where it was before the panel opened, else to the thread.
  const refocusOnClose = (panel: TogglePanel) => {
    const active = document.activeElement
    const before = focusBefore.current[panel]
    delete focusBefore.current[panel]
    if (!active?.closest(PANEL_SELECTOR[panel])) return
    requestAnimationFrame(() => {
      if (before instanceof HTMLElement && before.isConnected && !before.closest(PANEL_SELECTOR[panel])) before.focus()
      if (document.activeElement !== before) focus('thread')
    })
  }

  const setRightOpen = useCallback(
    (open: boolean) => {
      if (docked.right) return updatePrefs({ rightOpen: open })
      moveOverlay(open ? { open: 'right' } : { close: 'right' })
      // A right panel the sidebar pushed over the thread would dock again, open, when the sidebar closes.
      if (!open) updatePrefs({ rightOpen: false })
    },
    [docked.right, moveOverlay, updatePrefs],
  )

  const showHome = useCallback(() => {
    moveOverlay('thread-opened')
    if (project) selectHome(project.key)
  }, [moveOverlay, project, selectHome])

  // The focus asked for in the chat column gives it its room back, in the same render, so the focus lands there.
  const panelKeyRef = useRef(panelKey)
  panelKeyRef.current = panelKey
  useEffect(() => onFocus((target) => updatePanel(panelKeyRef.current, (st) => panelOnFocus(st, target))), [onFocus])

  const showPanelView = useCallback(
    (view: PanelTabId, opts: { expanded?: boolean; thread?: string } = {}) => {
      setRightOpen(true)
      updatePanel(opts.thread ?? panelKey, (st) => {
        const next = openTab(st, view)
        return opts.expanded === undefined || !docked.right ? next : { ...next, expanded: opts.expanded }
      })
    },
    [setRightOpen, panelKey, docked.right],
  )

  const focusPanel = useCallback(
    (panel: Panel) => {
      if (panel === 'right') return
      if (panel === 'sidebar') setSidebarOpen(true)
      if (panel === 'centre') moveOverlay('thread-focused')
      focus(panel === 'centre' ? 'thread' : panel)
    },
    [focus, moveOverlay, setSidebarOpen],
  )

  const togglePanel = (panel: TogglePanel) => {
    const setOpen = { sidebar: setSidebarOpen, right: setRightOpen }[panel]
    const shown = { sidebar: sidebarOpen, right: rightOpen }[panel]
    if (!shown) {
      const active = document.activeElement
      focusBefore.current[panel] = active && active !== document.body ? active : undefined
      setOpen(true)
      focus(PANEL_FOCUS[panel])
      return
    }
    refocusOnClose(panel)
    setOpen(false)
  }

  const value = useMemo<Layout>(
    () => ({
      mode,
      width,
      scale,
      theme,
      sidebarOpen,
      rightOpen,
      docked,
      overlay,
      widths: { sidebar: sidebar.width, right: right.width / 16 },
      panel,
      panelExpanded,
      zoom(step) {
        const next = zoomedScale(scale, step)
        report(() => setTextScale(next))
        if (zoomToast.current !== null) dismissToast(zoomToast.current)
        zoomToast.current = pushToast({ tone: 'info', text: `Text size ${Math.round(next * 100)}%` })
      },
      closeOverlay(why) {
        if (!overlay) return
        refocusOnClose(overlay)
        moveOverlay(why)
        if (overlay === 'right' && closesPanel(why)) updatePrefs({ rightOpen: false })
      },
      setWidth(side, rem, commit) {
        const limits = panelWidth(viewport, row)
        if (side === 'sidebar') {
          const rightPx = docked.right && prefs.rightOpen ? limits.min * scale + 1 : 0
          const limit = docked.sidebar ? Math.min(SIDEBAR_MAX_REM, sidebarMax(width, rightPx, root)) : SIDEBAR_MAX_REM
          const next = Math.max(Math.min(rem, limit), SIDEBAR_MIN_REM)
          setSidebarRem(next)
          if (commit) writeStored(SIDEBAR_WIDTH_KEY, next)
          return
        }
        const next = Math.max(Math.min(rem * 16, limits.max), limits.min) / 16
        if (commit) updatePrefs({ right: next })
        else setStored((all) => ({ ...all, [band]: { ...fresh(), ...all[band], right: next } }))
      },
      resetWidth(side) {
        if (side === 'right') return updatePrefs({ right: null })
        setSidebarRem(null)
        writeStored(SIDEBAR_WIDTH_KEY, null)
      },
      setRightOpen,
      toggleRight() {
        setRightOpen(!rightOpen)
      },
      togglePanel,
      showPanelView,
      togglePanelView(view, opts) {
        if (panelShows(rightOpen, panel, view)) return togglePanel('right')
        showPanelView(view, opts)
      },
      togglePanelExpanded() {
        if (!docked.right) return
        if (!rightOpen) setRightOpen(true)
        updatePanel(panelKey, (st) => ({ ...st, expanded: !rightOpen || !st.expanded }))
      },
      showPanelList() {
        updatePanel(panelKey, (st) => ({ ...st, active: null }))
      },
      closePanelTab(view) {
        updatePanel(panelKey, (st) => closeTab(st, view))
      },
      stepPanelTab(delta) {
        setRightOpen(true)
        updatePanel(panelKey, (st) => stepTab(st, delta))
      },
      setSidebarOpen,
      showHome,
      focusChat() {
        focus('thread')
      },
      focusPanel,
    }),
    [mode, width, scale, theme, sidebarOpen, rightOpen, prefs, prefs.rightOpen, docked, overlay, sidebar.width, right.width, panel, panelExpanded, panelKey, viewport, row, root, band, setSidebarOpen, setRightOpen, showHome, showPanelView, focusPanel, moveOverlay, refocusOnClose, updatePrefs, focus, report, setTextScale],
  )

  return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>
}
