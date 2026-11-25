import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useAgentos } from './AgentosContext'
import type { FocusTarget, SessionView } from './AgentosContext'
import { devFlags } from './api'
import { closesPanel, dockedMax, fit, MAX_REM, MIN_REM, modeFor, nextOverlay, SIDEBAR_DEFAULT_REM, sidebarPushesRightOver } from './panelLayout'
import type { Band, FitPrefs, LayoutMode, OverlayEvent, SidePanel } from './panelLayout'
import { readStored, writeStored } from './storage'
import { snapScale, zoomedScale } from './textScale'
import type { ThemeName } from './types'

type Appearance = 'dark' | 'light'
const DARK_QUERY = '(prefers-color-scheme: dark)'
export type Panel = 'sidebar' | 'centre' | 'right' | 'shell'
export { RAIL_REM, type SidePanel } from './panelLayout'
export type TogglePanel = 'sidebar' | 'shell' | 'right'

// The element that holds each panel and the focus target that moves the focus into it.
const PANEL_SELECTOR: Record<TogglePanel, string> = { sidebar: '[data-sidebar]', shell: '[data-panel="shell"]', right: '[data-panel="right"]' }
const PANEL_FOCUS: Record<TogglePanel, FocusTarget> = { sidebar: 'sidebar', shell: 'shell', right: 'right' }

const LAYOUT_KEY = 'agentos.layout.v3'
const SIDEBAR_WIDTH_KEY = 'agentos.sidebarWidth'
const PANEL_VIEW_KEY = 'agentos.panelView'
const SHELL_ROW_REM = 1.0325
const SHELL_PAD_REM = 0.5
export const shellRemForRows = (rows: number) => rows * SHELL_ROW_REM + SHELL_PAD_REM
const SHELL_DEFAULT_REM = shellRemForRows(6)
const SHELL_MIN_REM = shellRemForRows(3)

type BandPrefs = { sidebarOpen: boolean; rightOpen: boolean; shellOpen: boolean; right: number | null; shell: number | null }
type AllPrefs = Record<Band, BandPrefs>

const fresh = (): BandPrefs => ({ sidebarOpen: true, rightOpen: false, shellOpen: false, right: null, shell: null })

export type Layout = {
  mode: LayoutMode
  width: number
  scale: number
  theme: ThemeName
  sidebarOpen: boolean
  rightOpen: boolean
  shellOpen: boolean
  shellRem: number
  // Which side panels sit beside the thread; the others open over it.
  docked: Record<SidePanel, boolean>
  // The side panel open over the thread.
  overlay: SidePanel | null
  widths: Record<SidePanel, number>
  panelView: string
  statsOpen: boolean
  digestOpen: boolean
  zoom(step: -1 | 0 | 1): void
  closeOverlay(why: 'escape' | 'thread-focused'): void
  setWidth(panel: SidePanel, rem: number, commit: boolean): void
  resetWidth(panel: SidePanel): void
  setShellOpen(open: boolean): void
  setRightOpen(open: boolean): void
  toggleRight(): void
  togglePanel(panel: TogglePanel): void
  setPanelView(view: string): void
  showPanel(view: string): void
  showView(id: string, view: SessionView): void
  setShellHeight(rem: number, commit: boolean): void
  resetShellHeight(): void
  setSidebarOpen(open: boolean): void
  showHome(): void
  toggleStats(): void
  toggleDigest(): void
  openDigest(): void
  closeCentre(): void
  returnToTerminal(): void
  focusPanel(panel: Panel): void
}

const LayoutContext = createContext<Layout | null>(null)

export function useLayout(): Layout {
  const value = useContext(LayoutContext)
  if (!value) throw new Error('useLayout must be used inside LayoutProvider')
  return value
}

function useWindowSize(): { w: number; h: number } {
  const [size, setSize] = useState({ w: window.innerWidth, h: window.innerHeight })
  useEffect(() => {
    const update = () => setSize({ w: window.innerWidth, h: window.innerHeight })
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [])
  return size
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
  const { selectedId, project, settings, setTextScale, setSessionView, showHome: selectHome, focus, report, pushToast, dismissToast } = useAgentos('selectedId', 'project', 'settings', 'setTextScale', 'setSessionView', 'showHome', 'focus', 'report', 'pushToast', 'dismissToast')
  const { w: width, h: height } = useWindowSize()
  const mode = modeFor(width)
  const band: Band = mode === 'narrow' ? 'compact' : mode
  const scale = snapScale(settings.textScale)
  const systemAppearance = useSystemAppearance()
  const appearance = settings.theme === 'system' ? systemAppearance : settings.theme
  const theme = appearance === 'dark' ? settings.darkTheme : settings.lightTheme
  const [stored, setStored] = useState(() => readStored<Partial<AllPrefs>>(LAYOUT_KEY, {}))
  // Sidebar widths were saved per band before: until a width is saved on its own, the band the window opens in gives
  // the width the owner set. A saved null is a reset to the default.
  const [sidebarRem, setSidebarRem] = useState(() => {
    const saved = readStored<number | null | undefined>(SIDEBAR_WIDTH_KEY, undefined)
    return saved !== undefined ? saved : (readStored<Partial<Record<Band, { sidebar?: number | null }>>>(LAYOUT_KEY, {})[band]?.sidebar ?? null)
  })
  const [openOverlay, setOpenOverlay] = useState<SidePanel | null>(null)
  const [statsOpen, setStatsOpen] = useState(devFlags.view === 'stats')
  const [digestOpen, setDigestOpen] = useState(devFlags.view === 'digest')
  const [panelViews, setPanelViews] = useState(() => readStored<Record<string, string>>(PANEL_VIEW_KEY, {}))
  const zoomToast = useRef<number | null>(null)
  const focusBefore = useRef<Partial<Record<TogglePanel, Element>>>({})

  useLayoutEffect(() => {
    document.documentElement.style.setProperty('--ui-scale', String(scale))
  }, [scale])

  useLayoutEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])

  const prefs: BandPrefs = { ...fresh(), ...stored[band] }
  const fitPrefs: FitPrefs = { sidebarOpen: prefs.sidebarOpen, sidebar: sidebarRem ?? SIDEBAR_DEFAULT_REM, right: prefs.right }
  const { widths: fittedWidths, docked } = fit(width, fitPrefs, 16 * scale)
  const overlay = openOverlay && !docked[openOverlay] ? openOverlay : null
  const sidebarOpen = docked.sidebar ? prefs.sidebarOpen : overlay === 'sidebar'
  const rightOpen = docked.right ? prefs.rightOpen : overlay === 'right'
  const shellOpen = prefs.shellOpen
  const projectName = project?.name ?? ''
  const panelView = panelViews[projectName] ?? ''

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
    const first = previousSelected.current === null
    previousSelected.current = selectedId
    moveOverlay('thread-opened')
    if (first) return
    setStatsOpen(false)
    setDigestOpen(false)
  }, [selectedId, moveOverlay])

  const setSidebarOpen = useCallback(
    (open: boolean) => {
      if (!docked.sidebar) return moveOverlay(open ? { open: 'sidebar' } : { close: 'sidebar' })
      if (open && prefs.rightOpen && sidebarPushesRightOver(width, fitPrefs, 16 * scale)) moveOverlay({ open: 'right' })
      updatePrefs({ sidebarOpen: open })
    },
    [docked.sidebar, prefs.rightOpen, width, fitPrefs, scale, moveOverlay, updatePrefs],
  )

  // A panel closed another way (its close button, the other overlay) forgets where the focus was.
  useEffect(() => {
    const shown: Record<TogglePanel, boolean> = { sidebar: sidebarOpen, shell: shellOpen, right: rightOpen }
    for (const panel of Object.keys(shown) as TogglePanel[]) if (!shown[panel]) delete focusBefore.current[panel]
  }, [sidebarOpen, shellOpen, rightOpen])

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

  const setShellOpen = useCallback((open: boolean) => updatePrefs({ shellOpen: open }), [updatePrefs])

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
    setStatsOpen(false)
    setDigestOpen(false)
    moveOverlay('thread-opened')
    if (project) selectHome(project.key)
  }, [moveOverlay, project, selectHome])

  const setPanelView = useCallback(
    (view: string) =>
      setPanelViews((all) => {
        const next = { ...all, [projectName]: view }
        writeStored(PANEL_VIEW_KEY, next)
        return next
      }),
    [projectName],
  )

  const showPanel = useCallback(
    (view: string) => {
      setRightOpen(true)
      setPanelView(view)
    },
    [setRightOpen, setPanelView],
  )

  const focusPanel = useCallback(
    (panel: Panel) => {
      if (panel === 'right') return
      if (panel === 'sidebar') setSidebarOpen(true)
      if (panel === 'centre') moveOverlay('thread-focused')
      if (panel === 'shell') updatePrefs({ shellOpen: true })
      focus(panel === 'centre' ? 'thread' : panel)
    },
    [focus, moveOverlay, setSidebarOpen, updatePrefs],
  )

  const value = useMemo<Layout>(
    () => ({
      mode,
      width,
      scale,
      theme,
      sidebarOpen,
      rightOpen,
      shellOpen,
      shellRem: Math.min(Math.max(prefs.shell ?? SHELL_DEFAULT_REM, SHELL_MIN_REM), (0.6 * height) / (16 * scale)),
      docked,
      overlay,
      widths: { sidebar: fittedWidths.sidebar, right: fittedWidths.right },
      panelView,
      statsOpen,
      digestOpen,
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
      setWidth(panel, rem, commit) {
        const limit = docked[panel] ? Math.min(MAX_REM, dockedMax(panel, width, { ...fitPrefs, rightOpen: prefs.rightOpen }, 16 * scale)) : MAX_REM
        const next = Math.max(Math.min(rem, limit), MIN_REM[panel])
        if (panel === 'sidebar') {
          setSidebarRem(next)
          if (commit) writeStored(SIDEBAR_WIDTH_KEY, next)
        } else if (commit) updatePrefs({ right: next })
        else setStored((all) => ({ ...all, [band]: { ...fresh(), ...all[band], right: next } }))
      },
      resetWidth(panel) {
        if (panel === 'right') return updatePrefs({ right: null })
        setSidebarRem(null)
        writeStored(SIDEBAR_WIDTH_KEY, null)
      },
      setShellOpen,
      setRightOpen,
      toggleRight() {
        setRightOpen(!rightOpen)
      },
      togglePanel(panel) {
        const setOpen = { sidebar: setSidebarOpen, shell: setShellOpen, right: setRightOpen }[panel]
        const shown = { sidebar: sidebarOpen, shell: shellOpen, right: rightOpen }[panel]
        if (!shown) {
          const active = document.activeElement
          focusBefore.current[panel] = active && active !== document.body ? active : undefined
          setOpen(true)
          focus(PANEL_FOCUS[panel])
          return
        }
        refocusOnClose(panel)
        setOpen(false)
      },
      setPanelView,
      showPanel,
      showView(id, view) {
        showPanel('details')
        setSessionView(id, view)
      },
      setShellHeight(rem, commit) {
        const next = Math.max(rem, SHELL_MIN_REM)
        if (commit) updatePrefs({ shell: next })
        else setStored((all) => ({ ...all, [band]: { ...fresh(), ...all[band], shell: next } }))
      },
      resetShellHeight: () => updatePrefs({ shell: null }),
      setSidebarOpen,
      showHome,
      toggleStats() {
        setStatsOpen(!statsOpen)
        setDigestOpen(false)
        if (!statsOpen) moveOverlay('thread-opened')
      },
      toggleDigest() {
        setDigestOpen(!digestOpen)
        setStatsOpen(false)
        if (!digestOpen) moveOverlay('thread-opened')
      },
      openDigest() {
        setDigestOpen(true)
        setStatsOpen(false)
        moveOverlay('thread-opened')
      },
      closeCentre() {
        setStatsOpen(false)
        setDigestOpen(false)
      },
      returnToTerminal() {
        focus('thread')
      },
      focusPanel,
    }),
    [mode, width, scale, theme, sidebarOpen, rightOpen, shellOpen, prefs, prefs.rightOpen, prefs.shell, docked, overlay, fittedWidths.sidebar, fittedWidths.right, panelView, height, statsOpen, digestOpen, band, setSidebarOpen, setShellOpen, setRightOpen, showHome, setPanelView, showPanel, focusPanel, moveOverlay, refocusOnClose, updatePrefs, focus, report, setTextScale, setSessionView],
  )

  return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>
}
