import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useAgentos } from './AgentosContext'
import type { FocusTarget, SessionView } from './AgentosContext'
import { devFlags } from './api'
import { fit, MAX_REM, MIN_REM, SIDEBAR_DEFAULT_REM } from './panelLayout'
import type { FitPrefs, SidePanel } from './panelLayout'
import { readStored, writeStored } from './storage'
import { snapScale, zoomedScale } from './textScale'
import type { ThemeName } from './types'

type Appearance = 'dark' | 'light'
const DARK_QUERY = '(prefers-color-scheme: dark)'
export type Band = 'compact' | 'medium' | 'wide'
export type LayoutMode = 'narrow' | Band
export type Panel = 'sidebar' | 'centre' | 'right' | 'shell'
export type MobilePanel = 'thread' | 'panel' | 'threads' | 'shell'
export { RAIL_REM, type SidePanel } from './panelLayout'
export type TogglePanel = 'sidebar' | 'shell' | 'right'

// The element that holds each panel and the focus target that moves the focus into it.
const PANEL_SELECTOR: Record<TogglePanel, string> = { sidebar: '[data-sidebar]', shell: '[data-panel="shell"]', right: '[data-panel="right"]' }
const PANEL_FOCUS: Record<TogglePanel, FocusTarget> = { sidebar: 'sidebar', shell: 'shell', right: 'right' }

const TABS_BELOW = 640
const COMPACT_BELOW = 900
const MEDIUM_BELOW = 1250
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
  mobilePanel: MobilePanel
  widths: Record<SidePanel, number>
  panelView: string
  statsOpen: boolean
  digestOpen: boolean
  zoom(step: -1 | 0 | 1): void
  setMobilePanel(panel: MobilePanel): void
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

function modeFor(width: number): LayoutMode {
  if (width < TABS_BELOW) return 'narrow'
  if (width < COMPACT_BELOW) return 'compact'
  return width < MEDIUM_BELOW ? 'medium' : 'wide'
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
  const [mobilePanel, setMobilePanel] = useState<MobilePanel>('thread')
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
  const fitPrefs: FitPrefs = { ...prefs, sidebar: sidebarRem ?? SIDEBAR_DEFAULT_REM }
  const narrow = mode === 'narrow'
  const { widths: fittedWidths, rightForcedClosed } = fit(width, fitPrefs, 16 * scale)
  const sidebarOpen = narrow ? mobilePanel === 'threads' : prefs.sidebarOpen
  const rightOpen = !narrow && prefs.rightOpen && !rightForcedClosed
  const shellOpen = narrow ? mobilePanel === 'shell' : prefs.shellOpen
  const rightShown = narrow ? mobilePanel === 'panel' : rightOpen
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

  const previousSelected = useRef(selectedId)
  useEffect(() => {
    if (previousSelected.current === selectedId) return
    const first = previousSelected.current === null
    previousSelected.current = selectedId
    if (first) return
    setStatsOpen(false)
    setDigestOpen(false)
    if (narrow) setMobilePanel('thread')
  }, [selectedId, narrow])

  const setSidebarOpen = useCallback(
    (open: boolean) => {
      if (narrow) setMobilePanel(open ? 'threads' : 'thread')
      else updatePrefs({ sidebarOpen: open })
    },
    [narrow, updatePrefs],
  )

  // A panel closed another way (its close button, a mobile tab) forgets where the focus was.
  useEffect(() => {
    const shown: Record<TogglePanel, boolean> = { sidebar: sidebarOpen, shell: shellOpen, right: rightShown }
    for (const panel of Object.keys(shown) as TogglePanel[]) if (!shown[panel]) delete focusBefore.current[panel]
  }, [sidebarOpen, shellOpen, rightShown])

  const setShellOpen = useCallback(
    (open: boolean) => {
      if (narrow) setMobilePanel(open ? 'shell' : 'thread')
      else updatePrefs({ shellOpen: open })
    },
    [narrow, updatePrefs],
  )

  const setRightOpen = useCallback(
    (open: boolean) => {
      if (narrow) setMobilePanel(open ? 'panel' : 'thread')
      else updatePrefs({ rightOpen: open })
    },
    [narrow, updatePrefs],
  )

  const showHome = useCallback(() => {
    setStatsOpen(false)
    setDigestOpen(false)
    if (narrow) setMobilePanel('thread')
    if (project) selectHome(project.key)
  }, [narrow, project, selectHome])

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
      if (narrow) setMobilePanel('panel')
      else updatePrefs({ rightOpen: true })
      setPanelView(view)
    },
    [narrow, updatePrefs, setPanelView],
  )

  const focusPanel = useCallback(
    (panel: Panel) => {
      if (panel === 'right') return
      if (narrow) {
        if (panel === 'sidebar') setMobilePanel('threads')
        if (panel === 'centre') setMobilePanel('thread')
        if (panel === 'shell') setMobilePanel('shell')
      } else if (panel === 'sidebar') updatePrefs({ sidebarOpen: true })
      else if (panel === 'shell') updatePrefs({ shellOpen: true })
      focus(panel === 'centre' ? 'thread' : panel)
    },
    [focus, narrow, updatePrefs],
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
      mobilePanel,
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
      setMobilePanel,
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
        if (narrow) {
          setMobilePanel(mobilePanel === 'panel' ? 'thread' : 'panel')
          return
        }
        const open = !rightOpen
        if (open) checkRightFits(width, fitPrefs, 16 * scale)
        updatePrefs({ rightOpen: open })
      },
      togglePanel(panel) {
        const setOpen = { sidebar: setSidebarOpen, shell: setShellOpen, right: setRightOpen }[panel]
        const shown = { sidebar: sidebarOpen, shell: shellOpen, right: rightShown }[panel]
        const active = document.activeElement
        if (!shown) {
          if (panel === 'right' && !narrow) checkRightFits(width, fitPrefs, 16 * scale)
          focusBefore.current[panel] = active && active !== document.body ? active : undefined
          setOpen(true)
          focus(PANEL_FOCUS[panel])
          return
        }
        const before = focusBefore.current[panel]
        delete focusBefore.current[panel]
        setOpen(false)
        if (!active?.closest(PANEL_SELECTOR[panel])) return
        requestAnimationFrame(() => {
          if (before instanceof HTMLElement && before.isConnected && !before.closest(PANEL_SELECTOR[panel])) before.focus()
          if (document.activeElement !== before) focus('thread')
        })
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
        if (!statsOpen && narrow) setMobilePanel('thread')
      },
      toggleDigest() {
        setDigestOpen(!digestOpen)
        setStatsOpen(false)
        if (!digestOpen && narrow) setMobilePanel('thread')
      },
      openDigest() {
        setDigestOpen(true)
        setStatsOpen(false)
        if (narrow) setMobilePanel('thread')
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
    [mode, width, scale, theme, sidebarOpen, rightOpen, shellOpen, rightShown, narrow, prefs, fitPrefs, prefs.rightOpen, prefs.shell, mobilePanel, fittedWidths.sidebar, fittedWidths.right, panelView, height, statsOpen, digestOpen, band, setSidebarOpen, setShellOpen, setRightOpen, showHome, setPanelView, showPanel, focusPanel, updatePrefs, focus, report, setTextScale, setSessionView],
  )

  return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>
}
