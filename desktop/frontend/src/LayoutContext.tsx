import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useAgentos } from './AgentosContext'
import type { FocusTarget, SessionView } from './AgentosContext'
import { devFlags } from './api'
import { fit, MAX_REM, MIN_REM, modeFor, nextOverlay, SIDEBAR_DEFAULT_REM } from './panelLayout'
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
  // The side panel open over the thread on a narrow window.
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

function checkRightFits(width: number, prefs: FitPrefs, root: number) {
  if (fit(width, { ...prefs, rightOpen: true }, root).rightForcedClosed) throw 'The window is too narrow for the right panel'
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
  const [sidebarRem, setSidebarRem] = useState(() => readStored<number | null>(SIDEBAR_WIDTH_KEY, null))
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
  const narrow = mode === 'narrow'
  const fitPrefs: FitPrefs = { ...prefs, rightOpen: !narrow && prefs.rightOpen, sidebar: sidebarRem ?? SIDEBAR_DEFAULT_REM }
  const { widths: fittedWidths, rightForcedClosed } = fit(width, fitPrefs, 16 * scale)
  const overlay = narrow ? openOverlay : null
  const sidebarOpen = narrow ? overlay === 'sidebar' : prefs.sidebarOpen
  const rightOpen = narrow ? overlay === 'right' : prefs.rightOpen && !rightForcedClosed
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
    if (!narrow) setOpenOverlay(null)
  }, [narrow])

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
      if (narrow) moveOverlay(open ? { open: 'sidebar' } : { close: 'sidebar' })
      else updatePrefs({ sidebarOpen: open })
    },
    [narrow, moveOverlay, updatePrefs],
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
      if (narrow) moveOverlay(open ? { open: 'right' } : { close: 'right' })
      else updatePrefs({ rightOpen: open })
    },
    [narrow, moveOverlay, updatePrefs],
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
      if (narrow) moveOverlay({ open: 'right' })
      else updatePrefs({ rightOpen: true })
      setPanelView(view)
    },
    [narrow, moveOverlay, updatePrefs, setPanelView],
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
      },
      setWidth(panel, rem, commit) {
        const next = Math.min(Math.max(rem, MIN_REM[panel]), MAX_REM)
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
        const open = !rightOpen
        if (open && !narrow) checkRightFits(width, fitPrefs, 16 * scale)
        setRightOpen(open)
      },
      togglePanel(panel) {
        const setOpen = { sidebar: setSidebarOpen, shell: setShellOpen, right: setRightOpen }[panel]
        const shown = { sidebar: sidebarOpen, shell: shellOpen, right: rightOpen }[panel]
        if (!shown) {
          const active = document.activeElement
          if (panel === 'right' && !narrow) checkRightFits(width, fitPrefs, 16 * scale)
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
    [mode, width, scale, theme, sidebarOpen, rightOpen, shellOpen, narrow, prefs, fitPrefs, prefs.rightOpen, prefs.shell, overlay, fittedWidths.sidebar, fittedWidths.right, panelView, height, statsOpen, digestOpen, band, setSidebarOpen, setShellOpen, setRightOpen, showHome, setPanelView, showPanel, focusPanel, moveOverlay, refocusOnClose, updatePrefs, focus, report, setTextScale, setSessionView],
  )

  return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>
}
