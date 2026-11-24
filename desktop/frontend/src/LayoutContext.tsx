import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useAgentos } from './AgentosContext'
import type { HomePart } from './AgentosContext'
import { devFlags } from './api'
import { readStored, writeStored } from './storage'
import type { ThemeName } from './types'

type Appearance = 'dark' | 'light'
const DARK_QUERY = '(prefers-color-scheme: dark)'
export type Band = 'compact' | 'medium' | 'wide'
export type LayoutMode = 'narrow' | Band
export type Panel = 'sidebar' | 'centre' | 'right' | 'shell'
export type MobilePanel = 'queue' | 'notes' | 'session' | 'threads' | 'shell'
export type SidePanel = 'sidebar' | 'right'

export const SCALES = [0.85, 0.92, 1, 1.1, 1.2, 1.35, 1.5]
const DEFAULT_SCALE = 1
const TABS_BELOW = 640
const COMPACT_BELOW = 900
const MEDIUM_BELOW = 1250
const TERMINAL_MIN_PX = 280
const MAIN_PAD_PX = 24
const GAPS_PX = 24
const SHARE: Record<SidePanel, number> = { sidebar: 0.2, right: 0.24 }
const DEFAULT_RANGE: Record<SidePanel, [number, number]> = { sidebar: [15, 22], right: [9.5, 22] }
const MIN_REM: Record<SidePanel, number> = { sidebar: 13, right: 9.5 }
const MAX_REM = 32
const LAYOUT_KEY = 'agentos.layout.v3'
const SHELL_ROW_REM = 1.0325
const SHELL_PAD_REM = 0.5
export const shellRemForRows = (rows: number) => rows * SHELL_ROW_REM + SHELL_PAD_REM
const SHELL_DEFAULT_REM = shellRemForRows(6)
const SHELL_MIN_REM = shellRemForRows(3)

type BandPrefs = { sidebarOpen: boolean; rightOpen: boolean; shellOpen: boolean; sidebar: number | null; right: number | null; shell: number | null }
type AllPrefs = Record<Band, BandPrefs>

const fresh = (): BandPrefs => ({ sidebarOpen: true, rightOpen: false, shellOpen: true, sidebar: null, right: null, shell: null })

export type NewsPage = 'index' | 'tldr' | 'digest'

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
  statsOpen: boolean
  newsOpen: boolean
  newsPage: NewsPage
  zoom(step: -1 | 0 | 1): void
  setMobilePanel(panel: MobilePanel): void
  setWidth(panel: SidePanel, rem: number, commit: boolean): void
  resetWidth(panel: SidePanel): void
  setShellOpen(open: boolean): void
  setShellHeight(rem: number, commit: boolean): void
  resetShellHeight(): void
  toggleSidebar(): void
  togglePanels(): void
  setSidebarOpen(open: boolean): void
  showHome(part: HomePart): void
  toggleStats(): void
  toggleDigest(): void
  toggleNews(): void
  openNews(page: NewsPage): void
  closeNewsPage(): void
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

const clamp = (value: number, [min, max]: [number, number]) => Math.min(Math.max(value, min), max)

type Fitted = { widths: Record<SidePanel, number>; rightForcedClosed: boolean }

function fit(width: number, prefs: BandPrefs, root: number): Fitted {
  const preferred = (panel: SidePanel) => prefs[panel] ?? clamp((SHARE[panel] * width) / root, DEFAULT_RANGE[panel])
  let sidebar = prefs.sidebarOpen ? preferred('sidebar') * root : 0
  let right = prefs.rightOpen ? preferred('right') * root : 0
  const over = () => MAIN_PAD_PX + GAPS_PX + sidebar + right + TERMINAL_MIN_PX - width
  if (over() > 0 && prefs.rightOpen) right = Math.max(MIN_REM.right * root, right - over())
  if (over() > 0 && prefs.sidebarOpen) sidebar = Math.max(MIN_REM.sidebar * root, sidebar - over())
  const forced = over() > 0 && prefs.rightOpen
  if (forced) right = 0
  return { widths: { sidebar: sidebar / root, right: right / root }, rightForcedClosed: forced }
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
  const { selectedId, project, settings, setTextScale, showHome: selectHome, focus, report, pushToast, dismissToast } = useAgentos('selectedId', 'project', 'settings', 'setTextScale', 'showHome', 'focus', 'report', 'pushToast', 'dismissToast')
  const { w: width, h: height } = useWindowSize()
  const mode = modeFor(width)
  const band: Band = mode === 'narrow' ? 'compact' : mode
  const scale = settings.textScale
  const systemAppearance = useSystemAppearance()
  const appearance = settings.theme === 'system' ? systemAppearance : settings.theme
  const theme = appearance === 'dark' ? settings.darkTheme : settings.lightTheme
  const [stored, setStored] = useState(() => readStored<Partial<AllPrefs>>(LAYOUT_KEY, {}))
  const [mobilePanel, setMobilePanel] = useState<MobilePanel>('session')
  const [statsOpen, setStatsOpen] = useState(devFlags.view === 'stats')
  const [newsOpen, setNewsOpen] = useState(devFlags.view === 'news' || devFlags.view === 'digest')
  const [newsPage, setNewsPage] = useState<NewsPage>(devFlags.view === 'digest' ? 'digest' : 'index')
  const zoomToast = useRef<number | null>(null)

  useLayoutEffect(() => {
    document.documentElement.style.setProperty('--ui-scale', String(scale))
  }, [scale])

  useLayoutEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])

  const prefs: BandPrefs = { ...fresh(), ...stored[band] }
  const { widths: fittedWidths, rightForcedClosed } = fit(width, prefs, 16 * scale)
  const narrow = mode === 'narrow'
  const sidebarOpen = narrow ? mobilePanel === 'threads' : prefs.sidebarOpen
  const rightOpen = !narrow && prefs.rightOpen && !rightForcedClosed

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
    setNewsOpen(false)
    if (narrow) setMobilePanel('session')
  }, [selectedId, narrow])

  const setSidebarOpen = useCallback(
    (open: boolean) => {
      if (narrow) setMobilePanel(open ? 'threads' : 'session')
      else updatePrefs({ sidebarOpen: open })
    },
    [narrow, updatePrefs],
  )

  const showHome = useCallback(
    (part: HomePart) => {
      setStatsOpen(false)
      setNewsOpen(false)
      if (narrow) setMobilePanel(part === 'composer' ? 'session' : part)
      if (project && (!narrow || part === 'composer')) selectHome(project.key)
    },
    [narrow, project, selectHome],
  )

  const focusPanel = useCallback(
    (panel: Panel) => {
      if (panel === 'right') return
      if (narrow) {
        if (panel === 'sidebar') setMobilePanel('threads')
        if (panel === 'centre') setMobilePanel('session')
        if (panel === 'shell') setMobilePanel('shell')
      } else if (panel === 'sidebar') updatePrefs({ sidebarOpen: true })
      else if (panel === 'shell') updatePrefs({ shellOpen: true })
      focus(panel === 'centre' ? 'terminal' : panel)
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
      shellOpen: narrow ? mobilePanel === 'shell' : prefs.shellOpen,
      shellRem: Math.min(Math.max(prefs.shell ?? SHELL_DEFAULT_REM, SHELL_MIN_REM), (0.6 * height) / (16 * scale)),
      mobilePanel,
      widths: { sidebar: fittedWidths.sidebar, right: fittedWidths.right },
      statsOpen,
      newsOpen,
      newsPage,
      zoom(step) {
        const at = SCALES.indexOf(scale)
        const next = step === 0 ? DEFAULT_SCALE : SCALES[Math.min(Math.max((at < 0 ? 2 : at) + step, 0), SCALES.length - 1)]
        report(() => setTextScale(next))
        if (zoomToast.current !== null) dismissToast(zoomToast.current)
        zoomToast.current = pushToast({ tone: 'info', text: `Text size ${Math.round(next * 100)}%` })
      },
      setMobilePanel,
      setWidth(panel, rem, commit) {
        const next = Math.min(Math.max(rem, MIN_REM[panel]), MAX_REM)
        if (commit) updatePrefs({ [panel]: next })
        else setStored((all) => ({ ...all, [band]: { ...fresh(), ...all[band], [panel]: next } }))
      },
      resetWidth: (panel) => updatePrefs({ [panel]: null }),
      setShellOpen(open) {
        if (narrow) setMobilePanel(open ? 'shell' : 'session')
        else updatePrefs({ shellOpen: open })
      },
      setShellHeight(rem, commit) {
        const next = Math.max(rem, SHELL_MIN_REM)
        if (commit) updatePrefs({ shell: next })
        else setStored((all) => ({ ...all, [band]: { ...fresh(), ...all[band], shell: next } }))
      },
      resetShellHeight: () => updatePrefs({ shell: null }),
      toggleSidebar: () => setSidebarOpen(!sidebarOpen),
      togglePanels() {
        if (narrow) {
          setMobilePanel('session')
          return
        }
        const open = !sidebarOpen && !prefs.shellOpen
        updatePrefs({ sidebarOpen: open, shellOpen: open })
        if (!open) focus('terminal')
      },
      setSidebarOpen,
      showHome,
      toggleStats() {
        setStatsOpen(!statsOpen)
        setNewsOpen(false)
        if (!statsOpen && narrow) setMobilePanel('session')
      },
      toggleNews() {
        const close = newsOpen && newsPage === 'index'
        setNewsPage('index')
        setNewsOpen(!close)
        setStatsOpen(false)
        if (!close && narrow) setMobilePanel('session')
      },
      toggleDigest() {
        const close = newsOpen && newsPage === 'digest'
        setNewsPage('digest')
        setNewsOpen(!close)
        setStatsOpen(false)
        if (!close && narrow) setMobilePanel('session')
      },
      openNews(page) {
        setNewsPage(page)
        setNewsOpen(true)
        setStatsOpen(false)
        if (narrow) setMobilePanel('session')
      },
      closeNewsPage: () => setNewsPage('index'),
      closeCentre() {
        setStatsOpen(false)
        setNewsOpen(false)
      },
      returnToTerminal() {
        focus('terminal')
      },
      focusPanel,
    }),
    [mode, width, scale, theme, sidebarOpen, rightOpen, narrow, prefs.shellOpen, prefs.shell, mobilePanel, fittedWidths.sidebar, fittedWidths.right, height, statsOpen, newsOpen, newsPage, band, setSidebarOpen, showHome, focusPanel, updatePrefs, focus, report, setTextScale],
  )

  return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>
}
