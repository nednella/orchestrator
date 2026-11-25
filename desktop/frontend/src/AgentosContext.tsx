import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import type { ReactNode } from 'react'
import { ISSUES_DISABLED, api, devFlags, errorMessage, on } from './api'
import { answeredLate } from './chatQuestions'
import { EMPTY_DRAFT, readDraft, saveDraft } from './composerMarkdown'
import type { ComposerDraft } from './composerMarkdown'
import { readStored, removeStored, writeStored } from './storage'
import { liveThreads, sidebarThreads } from './threads'
import type { BrowserState, Cleanup, CleanupEvent, CleanupMode, DarkTheme, Digest, DigestSchedule, Evidence, Issue, LightTheme, Memory, Note, Pending, Project, Release, Session, PromptSend, Settings, Snapshot, ThemeSetting, Warning, ProjectList, ThreadStart } from './types'

export type Toast = {
  key: number
  tone: 'waiting' | 'replied' | 'opened' | 'pr' | 'evidence' | 'error' | 'info' | 'done'
  text: string
  detail?: string
  sessionId?: string
  view?: SessionView
  sticky?: boolean
  action?: { label: string; run(): void }
}

export type SessionView = 'pr' | 'evidence' | 'browser'
export const SESSION_VIEWS: SessionView[] = ['pr', 'evidence', 'browser']

export type Overlay = 'shortcuts' | 'settings' | 'patch-notes' | 'changelog' | null
export type FocusTarget = 'terminal' | 'shell' | 'sidebar' | 'right' | 'thread-search' | 'queue-filter' | 'note-jot' | 'composer'
export type ComposerKind = 'session' | 'chat'
export type ComposerMode = 'thread' | 'note'
export type PendingImage = { base64: string; mime: string }
// A thread's composer text, and the seq of the last draft agentos left in it that the composer has shown.
export type ThreadDraft = { draft: ComposerDraft; seen: number }

type AgentosState = {
  project: Project | null
  projects: Project[]
  sessions: Session[]
  projectSessions: Session[]
  selectedId: string | null
  issues: Issue[]
  issuesLoading: boolean
  issuesDisabled: boolean
  issueFilter: string
  notes: Note[]
  cleanups: Cleanup[]
  evidence: Record<string, Evidence[]>
  browserStates: Record<string, BrowserState>
  // The approval each live thread waits on, by thread id.
  pending: Record<string, Pending>
  digest: Digest | null
  digestSeen: number
  digestUnseenCount: number
  awake: boolean
  memory: Memory | null
  settings: Settings
  toasts: Toast[]
  overlay: Overlay
  composerDraft: ComposerDraft
  composerMode: ComposerMode
  composerKind: ComposerKind
  focusRequest: { target: FocusTarget; n: number }
  shellIds: string[]
  shellId: string
  version: string
  machine: string
  update: string
  updating: boolean
  patchNotes: Release[]
  viewOf(id: string | null): SessionView
}

// The actions keep their identity for the app's whole life, so picking one never renders a component again.
type AgentosActions = {
  openShell(): Promise<void>
  newShell(): Promise<void>
  closeShell(id: string): Promise<void>
  selectShell(id: string): void
  applyUpdate(): Promise<void>
  select(id: string): void
  showHome(projectKey: string): void
  detach(): void
  stepSession(delta: number): void
  newSession(start: ThreadStart): Promise<Session>
  newChat(start: ThreadStart): Promise<Session>
  killSession(id: string): Promise<void>
  dismissSession(id: string): Promise<void>
  resumeSession(id: string): Promise<Session>
  renameSession(id: string, title: string): Promise<void>
  startIssue(number: number, action?: string, background?: boolean, start?: ThreadStart): Promise<Session>
  switchProject(name: string, homeUnlessRemembered?: boolean): Promise<void>
  addProject(dir?: string): Promise<void>
  newProject(name: string): Promise<void>
  removeProject(name: string): Promise<void>
  setUpProject(): Promise<void>
  dismissSetup(): Promise<void>
  refreshIssues(): Promise<void>
  addNote(text: string, images?: PendingImage[]): Promise<void>
  updateNote(id: string, text: string): Promise<void>
  setNotePinned(id: string, pinned: boolean): Promise<void>
  setNoteArchived(id: string, archived: boolean): Promise<void>
  addNoteImage(id: string, image: PendingImage): Promise<void>
  removeNoteImage(id: string, url: string): Promise<void>
  deleteNote(id: string): Promise<void>
  refreshPRs(): Promise<void>
  loadBrowserState(id: string): Promise<void>
  setSessionView(id: string, view: SessionView): void
  cycleSessionView(delta: -1 | 1): void
  openBrowser(id: string, url?: string): Promise<void>
  runDigest(): Promise<void>
  digestToNote(itemId: string): Promise<void>
  dismissDigestItem(itemId: string): Promise<void>
  markDigestSeen(): void
  ackPR(id: string): Promise<void>
  openPR(session: Session): Promise<void>
  typeInto(id: string, text: string): Promise<void>
  answer(id: string, request: string, allow: boolean, reason?: string): Promise<void>
  answerQuestion(id: string, request: string, input: string): Promise<void>
  cleanupSession(id: string, force: boolean): Promise<void>
  noteToIssue(id: string): Promise<void>
  noteToSession(id: string): Promise<void>
  nextAttention(): void
  setComposerDraft(draft: ComposerDraft): void
  threadDraft(id: string): ThreadDraft
  setThreadDraft(id: string, draft: ThreadDraft): void
  setOverlay(overlay: Overlay): void
  setTheme(theme: ThemeSetting): Promise<void>
  setThemes(dark: DarkTheme, light: LightTheme): Promise<void>
  setCleanup(event: CleanupEvent, mode: CleanupMode): Promise<void>
  setTextScale(scale: number): Promise<void>
  setKeepAwake(on: boolean): Promise<void>
  setAgent(command: string): Promise<void>
  setPromptSend(mode: PromptSend): Promise<void>
  setBrowserEnabled(on: boolean): Promise<void>
  setDigestSchedule(schedule: DigestSchedule): Promise<void>
  setComposerMode(mode: ComposerMode): void
  setComposerKind(kind: ComposerKind): void
  setIssueFilter(query: string): void
  focus(target: FocusTarget): void
  report(run: () => unknown): void
  pushToast(toast: Omit<Toast, 'key'>): number
  dismissToast(key: number): void
}

export type Agentos = AgentosState & AgentosActions

type Store = { get(): Agentos; set(value: Agentos): void; subscribe(listener: () => void): () => void }

function createStore(initial: Agentos): Store {
  let value = initial
  const listeners = new Set<() => void>()
  return {
    get: () => value,
    set(next) {
      if (next === value) return
      value = next
      listeners.forEach((listener) => listener())
    },
    subscribe(listener) {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
  }
}

const AgentosContext = createContext<Store | null>(null)

function useStore(): Store {
  const store = useContext(AgentosContext)
  if (!store) throw new Error('useAgentos must be used inside AgentosProvider')
  return store
}

// A component renders again only when one of the keys it names changes; naming none takes them all.
export function useAgentos(): Agentos
export function useAgentos<K extends keyof Agentos>(...keys: K[]): Pick<Agentos, K>
export function useAgentos<K extends keyof Agentos>(...keys: K[]): Agentos | Pick<Agentos, K> {
  const store = useStore()
  const picked = useRef<Pick<Agentos, K> | null>(null)
  return useSyncExternalStore(store.subscribe, () => {
    const value = store.get()
    if (keys.length === 0) return value
    const last = picked.current
    if (last && keys.every((key) => Object.is(last[key], value[key]))) return last
    picked.current = Object.fromEntries(keys.map((key) => [key, value[key]])) as Pick<Agentos, K>
    return picked.current
  })
}

// `select` must return a value the state holds, or a primitive: a new object on each call renders forever.
export function useAgentosSelector<T>(select: (a: Agentos) => T): T {
  const store = useStore()
  return useSyncExternalStore(store.subscribe, () => select(store.get()))
}

// For handlers that need the state at the moment they run, without rendering on each change.
export function useLatestAgentos(): () => Agentos {
  return useStore().get
}

const cleanupToast = (entry: Cleanup): Omit<Toast, 'key'> => {
  if (!entry.merged) return { tone: 'info', text: `${entry.issue > 0 ? `#${entry.issue}` : entry.sessionTitle} cleaned up` }
  return {
    tone: 'done',
    text: "It's looking good brev",
    detail: `Shipped! ${entry.sessionTitle} · PR #${entry.pr} merged · worktree and branch cleaned up`,
  }
}

const WARNING_SOURCES: Record<Warning['source'], string> = { tmux: 'tmux', github: 'GitHub', 'pull requests': 'Pull requests', worktrees: 'Worktrees', config: 'Config', agent: 'Agent' }

const digestKey = (project: string) => `agentos.digestSeen.${project}`

const filterKey = (project: string) => `agentos.filter.${project}`
const draftKey = (project: string) => `agentos.draft.${project}`
const composerModeKey = (project: string) => `agentos.composerMode.${project}`
const threadDraftKey = (id: string) => `agentos.thread.draft.${id}`
const draftSeenKey = (id: string) => `agentos.thread.draftSeen.${id}`

const sameJSON = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b)

// Keeps each item of prev that next holds unchanged, and prev itself when nothing changed, so the rows
// and memos built on an item render again only when that item changed.
function keepUnchanged<T>(prev: T[], next: T[], keyOf: (item: T) => string | number): T[] {
  const byKey = new Map(prev.map((item) => [keyOf(item), item]))
  const kept = next.map((item) => {
    const old = byKey.get(keyOf(item))
    return old !== undefined && sameJSON(old, item) ? old : item
  })
  return kept.length === prev.length && kept.every((item, i) => item === prev[i]) ? prev : kept
}

type AgentosProviderProps = { children: ReactNode }

export function AgentosProvider({ children }: AgentosProviderProps) {
  const [project, setProject] = useState<Project | null>(null)
  const [projects, setProjects] = useState<Project[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [rawIssues, setRawIssues] = useState<Issue[]>([])
  const [issuesLoading, setIssuesLoading] = useState(true)
  const [issuesDisabled, setIssuesDisabled] = useState(false)
  const [issueFilter, setIssueFilterState] = useState('')
  const [notes, setNotes] = useState<Note[]>([])
  const [cleanups, setCleanups] = useState<Cleanup[]>([])
  const [evidence, setEvidence] = useState<Record<string, Evidence[]>>({})
  const [browserStates, setBrowserStates] = useState<Record<string, BrowserState>>({})
  const [pending, setPending] = useState<Record<string, Pending>>({})
  const [views, setViews] = useState<Record<string, SessionView>>({})
  const [digest, setDigest] = useState<Digest | null>(null)
  const [digestSeen, setDigestSeen] = useState(0)
  const [awake, setAwake] = useState(false)
  const [memory, setMemory] = useState<Memory | null>(null)
  const [settings, setSettings] = useState<Settings>({ theme: 'system', darkTheme: 'black', lightTheme: 'light', textScale: 1, keepAwake: true, agent: 'claude', agents: ['claude'], cleanup: { merge: 'auto', close: 'manual' }, promptSend: 'auto', browserEnabled: true, digestSchedule: 'weekly', dataDir: '', dataDirFixed: true })
  const [shellIds, setShellIds] = useState<string[]>([])
  const [pickedShell, setPickedShell] = useState('')
  const shellId = shellIds.includes(pickedShell) ? pickedShell : (shellIds[0] ?? '')
  const [version, setVersion] = useState('')
  const [machine, setMachine] = useState('')
  const [update, setUpdate] = useState('')
  const [updating, setUpdating] = useState(false)
  const [patchNotes, setPatchNotes] = useState<Release[]>([])
  const [toasts, setToasts] = useState<Toast[]>([])
  const [overlay, setOverlay] = useState<Overlay>((devFlags.overlay as Overlay) ?? null)
  const [composerDraft, setComposerDraftState] = useState(EMPTY_DRAFT)
  const [composerMode, setComposerModeState] = useState<ComposerMode>('thread')
  const [composerKind, setComposerKind] = useState<ComposerKind>('session')
  const [focusRequest, setFocusRequest] = useState<Agentos['focusRequest']>({ target: 'terminal', n: 0 })

  const sessionsRef = useRef(sessions)
  const projectsRef = useRef(projects)
  const selectedRef = useRef(selectedId)
  const projectRef = useRef(project)
  const lastSelected = useRef(new Map<string, string>())
  const toastKey = useRef(0)
  const toastTimers = useRef(new Map<number, number>())
  const projectEpoch = useRef(0)
  const lastCleanupToast = useRef(0)
  const warningToasts = useRef(new Map<Warning['source'], { message: string; key: number }>())
  const failedBrowserIds = useRef(new Set<string>())
  const detached = useRef(false)
  const shellIdsRef = useRef(shellIds)
  const notesRef = useRef(notes)
  const browserStatesRef = useRef(browserStates)
  const viewsRef = useRef(views)
  const issuesRef = useRef<Issue[]>([])
  sessionsRef.current = sessions
  projectsRef.current = projects
  projectRef.current = project
  shellIdsRef.current = shellIds
  notesRef.current = notes
  browserStatesRef.current = browserStates
  viewsRef.current = views

  const selectId = useCallback((id: string | null) => {
    selectedRef.current = id
    setSelectedId(id)
    if (!id) return
    detached.current = false
  }, [])

  const detach = useCallback(() => {
    const id = selectedRef.current
    if (!id) return
    detached.current = true
    selectId(null)
  }, [selectId])

  const focus = useCallback((target: FocusTarget) => setFocusRequest((r) => ({ target, n: r.n + 1 })), [])

  const dismissToast = useCallback((key: number) => {
    clearTimeout(toastTimers.current.get(key))
    toastTimers.current.delete(key)
    setToasts((list) => list.filter((t) => t.key !== key))
  }, [])

  const pushToast = useCallback(
    (toast: Omit<Toast, 'key'>) => {
      const key = ++toastKey.current
      setToasts((list) => [...list.slice(-2), { ...toast, key }])
      if (!toast.sticky) toastTimers.current.set(key, window.setTimeout(() => dismissToast(key), toast.tone === 'error' || toast.tone === 'done' ? 8000 : 5000))
      return key
    },
    [dismissToast],
  )

  useEffect(() => {
    const timers = toastTimers.current
    return () => timers.forEach((timer) => clearTimeout(timer))
  }, [])

  const report = useCallback(
    (run: () => unknown) => {
      const fail = (err: unknown) => pushToast({ tone: 'error', text: errorMessage(err) })
      try {
        const result = run()
        if (result instanceof Promise) result.catch(fail)
      } catch (err) {
        fail(err)
      }
    },
    [pushToast],
  )

  const selectAmong = useCallback(
    (list: Session[], homeUnlessRemembered = false) => {
      const current = selectedRef.current
      if (detached.current || (current && list.some((s) => s.id === current))) return
      const remembered = list.find((s) => s.id === lastSelected.current.get(projectRef.current?.name ?? ''))
      if (homeUnlessRemembered && !remembered) {
        detached.current = true
        return
      }
      const next = remembered ?? list[0]
      selectId(next ? next.id : null)
    },
    [selectId],
  )

  const applySessions = useCallback(
    (projectKey: string, items: Session[]) => {
      setSessions((prev) => keepUnchanged(prev, [...prev.filter((s) => s.project !== projectKey), ...items], (s) => s.id))
      if (projectKey === projectRef.current?.key) selectAmong(items)
    },
    [selectAmong],
  )

  const loadIssues = useCallback(async (refresh: boolean) => {
    const epoch = projectEpoch.current
    setIssuesLoading(true)
    try {
      const list = await api.issues(refresh)
      if (epoch !== projectEpoch.current) return
      setRawIssues(list)
      setIssuesDisabled(false)
    } catch (err) {
      if (errorMessage(err) !== ISSUES_DISABLED) throw err
      if (epoch !== projectEpoch.current) return
      setRawIssues([])
      setIssuesDisabled(true)
    } finally {
      if (epoch === projectEpoch.current) setIssuesLoading(false)
    }
  }, [])

  const enterProject = useCallback(
    (snap: Snapshot, epoch: number, homeUnlessRemembered = false) => {
      projectEpoch.current = epoch
      if (projectRef.current && selectedRef.current) lastSelected.current.set(projectRef.current.name, selectedRef.current)
      projectRef.current = snap.project
      detached.current = false
      selectId(null)
      setProject(snap.project)
      setProjects(snap.projects)
      setNotes(snap.notes)
      setShellIds(snap.shells)
      setVersion(snap.version)
      setMachine(snap.machine)
      setUpdate(snap.update)
      setRawIssues([])
      setIssuesDisabled(false)
      setIssueFilterState(readStored(filterKey(snap.project.name), ''))
      setComposerDraftState(readDraft(readStored<unknown>(draftKey(snap.project.name), '')))
      setComposerModeState(readStored<ComposerMode>(composerModeKey(snap.project.name), 'thread'))
      report(async () => {
        const list = await api.cleanups()
        if (epoch !== projectEpoch.current) return
        lastCleanupToast.current = list[0]?.at ?? 0
        setCleanups(list)
      })
      report(async () => {
        const next = await api.digest()
        if (epoch === projectEpoch.current) setDigest(next)
      })
      setDigestSeen(readStored(digestKey(snap.project.name), 0))
      setSessions((prev) => keepUnchanged(prev, snap.sessions, (s) => s.id))
      selectAmong(
        snap.sessions.filter((s) => s.project === snap.project.key),
        homeUnlessRemembered,
      )
      report(() => loadIssues(false))
    },
    [selectAmong, loadIssues, report, selectId],
  )

  // Each request takes the next epoch before it awaits, so only the latest one enters its project.
  const enterFrom = useCallback(
    async (request: Promise<Snapshot>, homeUnlessRemembered = false) => {
      const epoch = ++projectEpoch.current
      const snap = await request
      if (epoch === projectEpoch.current) enterProject(snap, epoch, homeUnlessRemembered)
      return snap
    },
    [enterProject],
  )

  // A failed GitHub step leaves the project in place; the user retries it or leaves the project local.
  const createRepo = useCallback(
    async (name: string) => {
      const key = pushToast({ tone: 'info', text: `Creating the GitHub repo for ${name}…`, sticky: true })
      try {
        const snap = await api.createRepo(name)
        if (snap.project.name !== projectRef.current?.name) return
        setProject(snap.project)
        setProjects(snap.projects)
        await loadIssues(true)
      } catch (err) {
        const failed = pushToast({
          tone: 'error',
          text: errorMessage(err),
          sticky: true,
          action: {
            label: 'Retry',
            run() {
              dismissToast(failed)
              report(() => createRepo(name))
            },
          },
        })
      } finally {
        dismissToast(key)
      }
    },
    [dismissToast, loadIssues, pushToast, report],
  )

  useEffect(() => {
    report(() => enterFrom(api.snapshot()))
    report(async () => setAwake(await api.awake()))
    report(async () => setMemory(await api.memory()))
    report(async () => setSettings(await api.settings()))
    report(async () => {
      const notes = await api.patchNotes()
      if (notes.length === 0) return
      setPatchNotes(notes)
      setOverlay((current) => current ?? 'patch-notes')
    })
  }, [enterFrom, report])

  // The project's settings follow the current project, which can change while the panel is shut.
  useEffect(() => {
    if (overlay === 'settings') report(async () => setSettings(await api.settings()))
  }, [overlay, report])

  // Events of a project can arrive after a switch to another one.
  const ifCurrent =
    <T,>(handler: (items: T[]) => void) =>
    (payload: ProjectList<T>) => {
      if (payload.project === projectRef.current?.key) handler(payload.items)
    }

  useEffect(() => on('sessions', ({ project: key, items }) => applySessions(key, items)), [applySessions])
  useEffect(() => on('term:exit', ({ id }) => setShellIds((ids) => ids.filter((x) => x !== id))), [])
  useEffect(() => on('notes', ifCurrent(setNotes)), [])
  useEffect(() => on('awake', setAwake), [])
  useEffect(() => on('memory', setMemory), [])
  useEffect(() => on('issues', ifCurrent(setRawIssues)), [])
  useEffect(() => on('update', ({ version }) => setUpdate(version)), [])
  useEffect(() => on('evidence', ({ id, items }) => setEvidence((map) => ({ ...map, [id]: items }))), [])
  useEffect(() => on('browser:state', (state) => setBrowserStates((map) => ({ ...map, [state.id]: state }))), [])
  useEffect(() => {
    // A thread's event that comes before the first read is newer than what the read gives for it.
    const heard = new Set<string>()
    const off = on('chat:pending', ({ id, pending: p }) => {
      heard.add(id)
      setPending((map) => {
        const { [id]: _, ...rest } = map
        return p ? { ...rest, [id]: p } : rest
      })
    })
    report(async () => {
      const read = (await api.chatPending()).filter((p) => !heard.has(p.id))
      setPending((map) => ({ ...Object.fromEntries(read.map((p) => [p.id, p])), ...map }))
    })
    return off
  }, [report])
  useEffect(
    () =>
      on('digest', (next) => {
        if (next.project === projectRef.current?.key) setDigest(next)
      }),
    [],
  )
  useEffect(
    () =>
      on(
        'cleanups',
        ifCurrent((list: Cleanup[]) => {
          setCleanups(list)
          const entry = list[0]
          if (!entry || entry.status !== 'done' || entry.at === lastCleanupToast.current) return
          lastCleanupToast.current = entry.at
          pushToast(cleanupToast(entry))
        }),
      ),
    [pushToast],
  )
  useEffect(
    () =>
      on('warnings', ({ source, message }: Warning) => {
        const shown = warningToasts.current.get(source)
        if (shown?.message === message) return
        if (shown) dismissToast(shown.key)
        warningToasts.current.delete(source)
        if (!message) return
        const key = pushToast({ tone: source === 'tmux' ? 'error' : 'info', text: `${WARNING_SOURCES[source]}: ${message}`, sticky: true })
        warningToasts.current.set(source, { message, key })
      }),
    [pushToast, dismissToast],
  )
  useEffect(
    () =>
      on('projects', (list) => {
        setProjects((prev) => keepUnchanged(prev, list, (p) => p.name))
        setProject((p) => {
          const next = list.find((x) => x.name === p?.name)
          return next && !sameJSON(next, p) ? next : p
        })
      }),
    [],
  )

  useEffect(
    () =>
      on('attention', ({ id, state }) => {
        if (id === selectedRef.current && state !== 'opened') return
        const session = sessionsRef.current.find((s) => s.id === id)
        if (!session) return
        const describe = (current: Session) => {
          if (state === 'waiting') return 'needs you'
          if (state === 'replied') return 'replied'
          if (state === 'opened') return current.pr ? `opened PR #${current.pr.number}` : 'opened a PR'
          if (state === 'evidence') return 'has something to show'
          return current.prAttention === 'checks' ? 'has failing checks' : 'has new review comments'
        }
        // The sessions event that carries prAttention lands in the same tick as this one.
        setTimeout(() => {
          const current = sessionsRef.current.find((s) => s.id === id) ?? session
          pushToast({ tone: state, text: `${current.title} ${describe(current)}`, sessionId: id, view: state === 'evidence' ? 'evidence' : undefined })
        }, 50)
      }),
    [pushToast],
  )

  useEffect(() => {
    if (!selectedId || evidence[selectedId]) return
    report(async () => {
      const items = await api.evidence(selectedId)
      setEvidence((map) => (map[selectedId] ? map : { ...map, [selectedId]: items }))
    })
  }, [selectedId, evidence, report])

  const loadBrowserState = useCallback(
    async (id: string) => {
      try {
        const state = await api.browserState(id)
        failedBrowserIds.current.delete(id)
        setBrowserStates((map) => (map[id] ? map : { ...map, [id]: state }))
      } catch (err) {
        if (failedBrowserIds.current.has(id)) return
        failedBrowserIds.current.add(id)
        pushToast({ tone: 'error', text: errorMessage(err) })
      }
    },
    [pushToast],
  )

  const digestUnseenCount = digest?.items.filter((i) => !i.noteId && i.at > digestSeen).length ?? 0

  const markDigestSeen = useCallback(() => {
    const now = Date.now()
    setDigestSeen(now)
    if (projectRef.current) writeStored(digestKey(projectRef.current.name), now)
  }, [])

  const projectSessions = useMemo(() => sessions.filter((s) => s.project === project?.key), [sessions, project?.key])

  const issues = useMemo(() => {
    const next = rawIssues.map((issue) => ({ ...issue, sessionId: projectSessions.find((s) => !s.chat && s.issue === issue.number)?.id ?? '' }))
    issuesRef.current = keepUnchanged(issuesRef.current, next, (issue) => issue.number)
    return issuesRef.current
  }, [rawIssues, projectSessions])

  const addSession = useCallback(
    (created: Session, background = false) => {
      setSessions((list) => (list.some((s) => s.id === created.id) ? list : [...list, created]))
      if (!background) {
        selectId(created.id)
        focus('terminal')
      }
      return created
    },
    [selectId, focus],
  )

  const inSidebarOrder = useCallback(() => {
    return sidebarThreads(sessionsRef.current, projectRef.current?.key)
  }, [])

  const selectSession = useCallback(
    (id: string) => {
      const target = sessionsRef.current.find((s) => s.id === id)
      const owner = target && target.project !== projectRef.current?.key ? projectsRef.current.find((p) => p.key === target.project) : undefined
      if (!owner) {
        selectId(id)
        focus('terminal')
        return
      }
      report(async () => {
        await enterFrom(api.switchProject(owner.name))
        if (projectRef.current?.key !== owner.key || !sessionsRef.current.some((s) => s.id === id)) return
        selectId(id)
        focus('terminal')
      })
    },
    [selectId, focus, report, enterFrom],
  )

  const viewOf = useCallback((id: string | null) => (id && views[id]) || 'pr', [views])

  // An answer the host refused because its approval no longer waits says so, and drops the approval from pending in
  // case the event that would have was missed.
  const answerFailed = useCallback((id: string, request: string, err: unknown): never => {
    if (!answeredLate(errorMessage(err))) throw err
    setPending((map) => {
      if (map[id]?.request !== request) return map
      const { [id]: _, ...rest } = map
      return rest
    })
    throw 'That request has already ended'
  }, [])

  const actions = useMemo<AgentosActions>(
    () => ({
      async applyUpdate() {
        setUpdating(true)
        try {
          await api.update()
        } catch (err) {
          setUpdating(false)
          throw err
        }
      },
      async openShell() {
        if (shellIdsRef.current.length) return
        const shell = await api.shellOpen(selectedRef.current ?? '')
        setShellIds([shell.id])
      },
      async newShell() {
        const shell = await api.shellNew(selectedRef.current ?? '')
        setShellIds((ids) => [...ids, shell.id])
        setPickedShell(shell.id)
      },
      async closeShell(id) {
        await api.shellClose(id)
        setShellIds((ids) => ids.filter((x) => x !== id))
      },
      selectShell: setPickedShell,
      select: selectSession,
      showHome(projectKey) {
        const owner = projectsRef.current.find((p) => p.key === projectKey)
        if (!owner) return
        const show = () => {
          detached.current = true
          selectId(null)
        }
        if (owner.key === projectRef.current?.key) return show()
        report(async () => {
          await enterFrom(api.switchProject(owner.name))
          if (projectRef.current?.key === owner.key) show()
        })
      },
      detach,
      stepSession(delta) {
        const list = inSidebarOrder()
        if (list.length === 0) return
        const at = list.findIndex((s) => s.id === selectedRef.current)
        selectSession(list[(at + delta + list.length) % list.length].id)
      },
      async newSession({ prompt, model, effort }) {
        return addSession(await api.newSession('', prompt, model, effort))
      },
      async newChat({ prompt, model, effort }) {
        return addSession(await api.newChat('', prompt, model, effort))
      },
      async killSession(id) {
        await api.killSession(id)
        if (selectedRef.current !== id) return
        const next = sessionsRef.current.find((s) => s.project === projectRef.current?.key && s.id !== id && s.state !== 'ended')
        if (next) selectId(next.id)
        else detach()
      },
      async dismissSession(id) {
        await api.dismissSession(id)
        removeStored(threadDraftKey(id))
        removeStored(draftSeenKey(id))
        const owner = sessionsRef.current.find((s) => s.id === id)?.project
        if (owner) applySessions(owner, sessionsRef.current.filter((s) => s.project === owner && s.id !== id))
      },
      async resumeSession(id) {
        return addSession(await api.resumeSession(id))
      },
      async renameSession(id, title) {
        const trimmed = title.trim()
        if (!trimmed) throw 'A title cannot be empty'
        await api.renameSession(id, trimmed)
        setSessions((list) => list.map((s) => (s.id === id ? { ...s, title: trimmed } : s)))
      },
      async startIssue(number, action = '', background = false, start) {
        return addSession(await api.startIssue(number, action, start?.prompt, start?.model, start?.effort), background)
      },
      async switchProject(name, homeUnlessRemembered) {
        if (name === projectRef.current?.name) return
        await enterFrom(api.switchProject(name), homeUnlessRemembered)
      },
      async addProject(dir) {
        await enterFrom(dir ? api.addProjectDir(dir) : api.addProject())
      },
      async newProject(name) {
        const key = pushToast({ tone: 'info', text: `Creating ${name}…`, sticky: true })
        const before = projectRef.current?.dir
        let snap: Snapshot
        try {
          snap = await enterFrom(api.newProject(name))
        } finally {
          dismissToast(key)
        }
        if (snap.project.dir !== before) await createRepo(snap.project.name)
      },
      async removeProject(name) {
        const epoch = projectEpoch.current
        const snap = await api.removeProject(name)
        if (epoch !== projectEpoch.current) return
        if (snap.project.name === projectRef.current?.name) setProjects(snap.projects)
        else enterProject(snap, ++projectEpoch.current)
      },
      async setUpProject() {
        try {
          addSession(await api.setUpProject())
        } finally {
          const snap = await api.snapshot()
          setProject(snap.project)
          setProjects(snap.projects)
          await loadIssues(true)
        }
      },
      async dismissSetup() {
        const snap = await api.dismissSetup()
        setProject(snap.project)
        setProjects(snap.projects)
      },
      refreshIssues: () => loadIssues(true),
      loadBrowserState,
      setSessionView(id, view) {
        setViews((map) => ({ ...map, [id]: view }))
      },
      cycleSessionView(delta) {
        const id = selectedRef.current
        if (!id) return
        const at = SESSION_VIEWS.indexOf(viewsRef.current[id] ?? 'pr')
        const next = SESSION_VIEWS[(at + delta + SESSION_VIEWS.length) % SESSION_VIEWS.length]
        setViews((map) => ({ ...map, [id]: next }))
      },
      async openBrowser(id, url) {
        setViews((map) => ({ ...map, [id]: 'browser' }))
        const state = browserStatesRef.current[id] ?? (await api.browserState(id))
        if (!state.open) {
          const opened = await api.browserOpen(id, url ?? '')
          setBrowserStates((map) => ({ ...map, [id]: opened }))
        } else if (url) {
          await api.browserGoto(id, url)
        }
      },
      async runDigest() {
        await api.runDigest()
      },
      async digestToNote(itemId) {
        await api.digestToNote(itemId)
      },
      async dismissDigestItem(itemId) {
        await api.dismissDigestItem(itemId)
      },
      markDigestSeen,
      async addNote(text, images = []) {
        const trimmed = text.trim()
        if (!trimmed && images.length === 0) throw 'A note cannot be empty'
        let note = await api.addNote(trimmed)
        for (const image of images) note = await api.addNoteImage(note.id, image.base64, image.mime)
        const saved = note
        setNotes((list) => (list.some((n) => n.id === saved.id) ? list.map((n) => (n.id === saved.id ? saved : n)) : [saved, ...list]))
      },
      async updateNote(id, text) {
        const trimmed = text.trim()
        if (!trimmed && !notesRef.current.find((n) => n.id === id)?.images.length) throw 'A note cannot be empty'
        const note = await api.updateNote(id, trimmed)
        setNotes((list) => list.map((n) => (n.id === id ? note : n)))
      },
      async setNotePinned(id, pinned) {
        const note = await api.setNotePinned(id, pinned)
        setNotes((list) => list.map((n) => (n.id === id ? note : n)))
      },
      async setNoteArchived(id, archived) {
        const note = await api.setNoteArchived(id, archived)
        setNotes((list) => list.map((n) => (n.id === id ? note : n)))
      },
      async addNoteImage(id, image) {
        const note = await api.addNoteImage(id, image.base64, image.mime)
        setNotes((list) => list.map((n) => (n.id === id ? note : n)))
      },
      async removeNoteImage(id, url) {
        const note = await api.removeNoteImage(id, url)
        setNotes((list) => list.map((n) => (n.id === id ? note : n)))
      },
      async refreshPRs() {
        await api.refreshPRs()
      },
      async ackPR(id) {
        await api.ackPR(id)
      },
      async openPR(session) {
        if (!session.pr) throw 'This session has no pull request yet'
        await api.openURL(session.pr.url)
        if (session.prAttention) await api.ackPR(session.id)
      },
      async typeInto(id, text) {
        await api.typeInto(id, text)
      },
      async answer(id, request, allow, reason = '') {
        await api.chatAnswer(id, request, allow, reason).catch((err) => answerFailed(id, request, err))
      },
      async answerQuestion(id, request, input) {
        await api.chatAnswerInput(id, request, input).catch((err) => answerFailed(id, request, err))
      },
      async cleanupSession(id, force) {
        await api.cleanup(id, force)
      },
      async deleteNote(id) {
        await api.deleteNote(id)
        setNotes((list) => list.filter((n) => n.id !== id))
      },
      async noteToIssue(id) {
        await api.noteToIssue(id)
        setNotes((list) => list.filter((n) => n.id !== id))
      },
      async noteToSession(id) {
        addSession(await api.noteToSession(id))
      },
      nextAttention() {
        const list = liveThreads(projectsRef.current, sessionsRef.current)
        const urgent = [...list.filter((s) => s.state === 'waiting'), ...list.filter((s) => s.state === 'idle')]
        if (urgent.length === 0) throw 'Nothing needs you right now'
        const at = urgent.findIndex((s) => s.id === selectedRef.current)
        selectSession(urgent[(at + 1) % urgent.length].id)
      },
      setComposerDraft(draft) {
        setComposerDraftState(draft)
        if (projectRef.current) writeStored(draftKey(projectRef.current.name), saveDraft(draft))
      },
      threadDraft(id) {
        return { draft: readDraft(readStored<unknown>(threadDraftKey(id), '')), seen: readStored(draftSeenKey(id), 0) }
      },
      setThreadDraft(id, { draft, seen }) {
        if (draft.text.trim() === '') removeStored(threadDraftKey(id))
        else writeStored(threadDraftKey(id), saveDraft(draft))
        writeStored(draftSeenKey(id), seen)
      },
      setOverlay,
      async setTheme(theme) {
        setSettings(await api.setTheme(theme))
      },
      async setThemes(dark, light) {
        setSettings(await api.setThemes(dark, light))
      },
      async setCleanup(event, mode) {
        setSettings(await api.setCleanup(event, mode))
      },
      async setTextScale(scale) {
        setSettings(await api.setTextScale(scale))
      },
      async setKeepAwake(on) {
        setSettings(await api.setKeepAwake(on))
      },
      async setAgent(command) {
        setSettings(await api.setAgent(command))
      },
      async setPromptSend(mode) {
        setSettings(await api.setPromptSend(mode))
      },
      async setBrowserEnabled(on) {
        setSettings(await api.setBrowserEnabled(on))
      },
      async setDigestSchedule(schedule) {
        setSettings(await api.setDigestSchedule(schedule))
      },
      setComposerMode(mode) {
        setComposerModeState(mode)
        if (projectRef.current) writeStored(composerModeKey(projectRef.current.name), mode)
      },
      setComposerKind,
      setIssueFilter(query) {
        setIssueFilterState(query)
        if (projectRef.current) writeStored(filterKey(projectRef.current.name), query)
      },
      focus,
      report,
      pushToast,
      dismissToast,
    }),
    [selectId, selectSession, inSidebarOrder, detach, addSession, applySessions, enterProject, enterFrom, createRepo, loadIssues, loadBrowserState, markDigestSeen, answerFailed, focus, report, pushToast, dismissToast],
  )

  const value = useMemo<Agentos>(
    () => ({
      project,
      projects,
      sessions,
      projectSessions,
      selectedId,
      issues,
      issuesLoading,
      issuesDisabled,
      issueFilter,
      notes,
      cleanups,
      evidence,
      browserStates,
      pending,
      digest,
      digestSeen,
      digestUnseenCount,
      awake,
      memory,
      settings,
      toasts,
      overlay,
      composerDraft,
      composerMode,
      composerKind,
      focusRequest,
      shellIds,
      shellId,
      version,
      machine,
      update,
      updating,
      patchNotes,
      viewOf,
      ...actions,
    }),
    [project, projects, sessions, projectSessions, selectedId, issues, issuesLoading, issuesDisabled, issueFilter, notes, cleanups, evidence, browserStates, pending, digest, digestSeen, digestUnseenCount, awake, memory, settings, toasts, overlay, composerDraft, composerMode, composerKind, focusRequest, shellIds, shellId, version, machine, update, updating, patchNotes, viewOf, actions],
  )

  const [store] = useState(() => createStore(value))
  useLayoutEffect(() => store.set(value), [store, value])

  return <AgentosContext.Provider value={store}>{children}</AgentosContext.Provider>
}
