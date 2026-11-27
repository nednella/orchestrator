import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import type { ReactNode } from 'react'
import { newThreadAccess } from './access'
import { ISSUES_DISABLED, api, devFlags, errorMessage, on } from './api'
import { answeredLate } from './chatQuestions'
import { back, current, forward, goneAgent, goneFrom, NO_HISTORY, projectOf, prune, push, stepTo, threadOf } from './navHistory'
import type { History, Known, View } from './navHistory'
import { forgetPanel } from './panelStore'
import { shellsChanged } from './projectTerminals'
import type { PanelViewId } from './panelTabs'
import { EMPTY_DRAFT, draftOf, fromMarkdown, readDraft, saveDraft } from './composerMarkdown'
import type { ComposerDraft } from './composerMarkdown'
import { readStored, removeStored, writeStored } from './storage'
import { descendantsOf, firstThread, goneThreads, nextThread, sidebarThreads, splitSideChats, threadTreeOf, urgentThreads } from './threads'
import type { ThreadTree } from './threads'
import { overlayAtStart } from './whatsNew'
import type { Access, Place, BrowserState, Cleanup, CleanupEvent, CleanupMode, DarkTheme, Digest, DigestSchedule, Evidence, Issue, LightTheme, Memory, Note, NewProject, Pending, Project, Release, Session, Shell, PromptSend, Settings, Snapshot, ThemeSetting, UpdateState, Warning, ProjectList, ThreadStart } from './types'

export type Toast = {
  key: number
  tone: 'waiting' | 'replied' | 'opened' | 'pr' | 'evidence' | 'error' | 'info' | 'done' | 'success' | 'warning'
  text: string
  detail?: string
  sessionId?: string
  view?: PanelViewId
  sticky?: boolean
  action?: { label: string; run(): void }
}

export type Overlay = 'shortcuts' | 'add-project' | 'settings' | 'changelog' | null
export type FocusTarget = 'thread' | 'sidebar' | 'right' | 'thread-search' | 'queue-filter' | 'note-jot' | 'composer'
export type ComposerMode = 'thread' | 'note'
export type PendingImage = { base64: string; mime: string }
// A thread's composer text, and the seq of the last draft orchestrator left in it that the composer has shown.
export type ThreadDraft = { draft: ComposerDraft; seen: number }

type OrchestratorState = {
  project: Project | null
  projects: Project[]
  // The threads of every project, without the side chats, which show only in their threads' right panels.
  sessions: Session[]
  sideChats: Session[]
  projectSessions: Session[]
  // Each thread's top-level thread and what each top-level thread needs the owner for, built once per sessions change.
  threadTree: ThreadTree
  selectedId: string | null
  // The views the owner went through, for Back and Forward; the one shown now may be a subagent's view of the selected
  // thread. Back and Forward are on while a view whose thread and project are still there lies that way.
  viewHistory: History
  canGoBack: boolean
  canGoForward: boolean
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
  // Why the digest could not be read; empty while it loads or once it is read.
  digestError: string
  digestSeen: number
  digestUnseenCount: number
  awake: boolean
  memory: Memory | null
  settings: Settings
  toasts: Toast[]
  overlay: Overlay
  composerDraft: ComposerDraft
  composerMode: ComposerMode
  composerAccess: Access
  focusRequest: { target: FocusTarget; n: number }
  shells: Shell[]
  machine: string
  update: UpdateState
  startNotes: Release[]
}

// The actions keep their identity for the app's whole life, so picking one never renders a component again.
type OrchestratorActions = {
  select(id: string): void
  showHome(projectKey: string): void
  // openAgent shows the view of a subagent Claude started in the thread's chat, in place of that chat.
  openAgent(thread: string, agent: string): void
  // closeGoneAgent takes the thread back to its chat once its loaded chat no longer holds the subagent shown.
  closeGoneAgent(thread: string, loaded: boolean, holds: boolean): void
  // goBack and goForward move through the views the owner went through, past the threads and projects now gone.
  goBack(): void
  goForward(): void
  detach(): void
  stepSession(delta: number): void
  // place '' leaves it to new_thread_place.
  newThread(start: ThreadStart, access: Access, place: Place | ''): Promise<Session>
  settleThread(id: string): Promise<void>
  // deleteThread stops the thread when it runs and removes it with its chat; with worktree, the worktree orchestrator made
  // for it goes too, unless a safety check keeps it, which refuses the delete. The branch stays.
  deleteThread(id: string, worktree: boolean): Promise<void>
  wakeThread(id: string): Promise<Session>
  // setAccess gives the thread the access between turns; the row shows it once the sessions say so.
  setAccess(id: string, access: Access): Promise<void>
  // openInEditor opens the folder the thread works in, its worktree or else the project folder, in the editor app.
  openInEditor(id: string): Promise<void>
  // forkSession starts a thread that continues the thread's conversation up to its reply at, with draft in the new
  // thread's composer, and selects it.
  forkSession(id: string, at: string, draft?: string): Promise<Session>
  // startSideChat starts a side chat of the thread, a fork of it at its latest finished reply, with text and images as
  // its first message when there are any. One start runs at a time.
  startSideChat(parent: string, text: string, images: PendingImage[]): Promise<Session>
  // deleteSideChat deletes the side chat; with unused, only when nobody sent it a message.
  deleteSideChat(id: string, unused: boolean): Promise<void>
  renameSession(id: string, title: string): Promise<void>
  startIssue(number: number, action?: string, background?: boolean, start?: ThreadStart): Promise<Session>
  switchProject(name: string, homeUnlessRemembered?: boolean): Promise<void>
  // addProject opens the folder as a project; with create it makes the folder first.
  addProject(dir: string, create?: boolean): Promise<void>
  // createProject makes a project from a name, opens it and focuses its composer, and publishes it to GitHub in the
  // background when asked. It says whether the project was made.
  createProject(name: string, publish: boolean): Promise<boolean>
  setUpProject(): Promise<void>
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
  openBrowser(id: string, url?: string): Promise<void>
  runDigest(): Promise<void>
  reloadDigest(): void
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
  setComposerDraft(draft: ComposerDraft, project?: string): void
  threadDraft(id: string): ThreadDraft
  setThreadDraft(id: string, draft: ComposerDraft): void
  setDraftsSeen(id: string, seq: number): void
  setOverlay(overlay: Overlay): void
  setTheme(theme: ThemeSetting): Promise<void>
  setThemes(dark: DarkTheme, light: LightTheme): Promise<void>
  setCleanup(event: CleanupEvent, mode: CleanupMode): Promise<void>
  setTextScale(scale: number): Promise<void>
  setKeepAwake(on: boolean): Promise<void>
  setProjectsDir(dir: string): Promise<void>
  setPromptSend(mode: PromptSend): Promise<void>
  setBrowserEnabled(on: boolean): Promise<void>
  setDigestSchedule(schedule: DigestSchedule): Promise<void>
  // setThreadAccess also gives the Home composer the new default.
  setThreadPlace(place: Place): Promise<void>
  setThreadAccess(access: Access): Promise<void>
  setComposerMode(mode: ComposerMode): void
  setComposerAccess(access: Access): void
  setIssueFilter(query: string): void
  focus(target: FocusTarget): void
  // onFocus calls listener with each focus target asked for, at the ask, until the returned function is called.
  onFocus(listener: (target: FocusTarget) => void): () => void
  report(run: () => unknown): void
  pushToast(toast: Omit<Toast, 'key'>): number
  dismissToast(key: number): void
}

export type Orchestrator = OrchestratorState & OrchestratorActions

type Store = { get(): Orchestrator; set(value: Orchestrator): void; subscribe(listener: () => void): () => void }

function createStore(initial: Orchestrator): Store {
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

const OrchestratorContext = createContext<Store | null>(null)

function useStore(): Store {
  const store = useContext(OrchestratorContext)
  if (!store) throw new Error('useOrchestrator must be used inside OrchestratorProvider')
  return store
}

// A component renders again only when one of the keys it names changes; naming none takes them all.
export function useOrchestrator(): Orchestrator
export function useOrchestrator<K extends keyof Orchestrator>(...keys: K[]): Pick<Orchestrator, K>
export function useOrchestrator<K extends keyof Orchestrator>(...keys: K[]): Orchestrator | Pick<Orchestrator, K> {
  const store = useStore()
  const picked = useRef<Pick<Orchestrator, K> | null>(null)
  return useSyncExternalStore(store.subscribe, () => {
    const value = store.get()
    if (keys.length === 0) return value
    const last = picked.current
    if (last && keys.every((key) => Object.is(last[key], value[key]))) return last
    picked.current = Object.fromEntries(keys.map((key) => [key, value[key]])) as Pick<Orchestrator, K>
    return picked.current
  })
}

// `select` must return a value the state holds, or a primitive: a new object on each call renders forever.
export function useOrchestratorSelector<T>(select: (a: Orchestrator) => T): T {
  const store = useStore()
  return useSyncExternalStore(store.subscribe, () => select(store.get()))
}

// For handlers that need the state at the moment they run, without rendering on each change.
export function useLatestOrchestrator(): () => Orchestrator {
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

const digestKey = (project: string) => `orchestrator.digestSeen.${project}`

const filterKey = (project: string) => `orchestrator.filter.${project}`
const draftKey = (project: string) => `orchestrator.draft.${project}`
const composerModeKey = (project: string) => `orchestrator.composerMode.${project}`
const threadDraftKey = (id: string) => `orchestrator.thread.draft.${id}`
const draftSeenKey = (id: string) => `orchestrator.thread.draftSeen.${id}`

// forgetThread drops what the app keeps of a thread that is gone: its draft, and its right panel.
function forgetThread(id: string) {
  removeStored(threadDraftKey(id))
  removeStored(draftSeenKey(id))
  forgetPanel(id)
}

// viewOf is the view that shows the thread, or with none the project's home.
const viewOf = (id: string | null, project: string): View => (id ? { kind: 'thread', id } : { kind: 'home', project })

// shownIn is whether Back or Forward can show the view now: its project and its thread are among projects and sessions.
const shownIn = (sessions: Session[], projects: Project[]) => (view: View) => projects.some((p) => p.key === projectOf(view)) && (view.kind === 'home' || sessions.some((s) => s.id === threadOf(view)))

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

type OrchestratorProviderProps = { children: ReactNode }

export function OrchestratorProvider({ children }: OrchestratorProviderProps) {
  const [project, setProject] = useState<Project | null>(null)
  const [projects, setProjects] = useState<Project[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [sideChats, setSideChats] = useState<Session[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [viewHistory, setViewHistory] = useState(NO_HISTORY)
  const [rawIssues, setRawIssues] = useState<Issue[]>([])
  const [issuesLoading, setIssuesLoading] = useState(true)
  const [issuesDisabled, setIssuesDisabled] = useState(false)
  const [issueFilter, setIssueFilterState] = useState('')
  const [notes, setNotes] = useState<Note[]>([])
  const [cleanups, setCleanups] = useState<Cleanup[]>([])
  const [evidence, setEvidence] = useState<Record<string, Evidence[]>>({})
  const [browserStates, setBrowserStates] = useState<Record<string, BrowserState>>({})
  const [pending, setPending] = useState<Record<string, Pending>>({})
  const [digest, setDigest] = useState<Digest | null>(null)
  const [digestError, setDigestError] = useState('')
  const [digestSeen, setDigestSeen] = useState(0)
  const [awake, setAwake] = useState(false)
  const [memory, setMemory] = useState<Memory | null>(null)
  const [settings, setSettings] = useState<Settings>({ theme: 'system', darkTheme: 'black', lightTheme: 'light', textScale: 1, keepAwake: true, cleanup: { merge: 'auto', close: 'manual' }, promptSend: 'auto', browserEnabled: true, digestSchedule: 'weekly', threadPlace: '', threadAccess: '', dataDir: '', dataDirFixed: true, editorApp: '', projectsDir: '' })
  const [shells, setShells] = useState<Shell[]>([])
  const [machine, setMachine] = useState('')
  const [update, setUpdate] = useState<UpdateState>({ status: 'idle', version: '', percent: 0, failedStep: '', error: '' })
  const [startNotes, setStartNotes] = useState<Release[]>([])
  const [toasts, setToasts] = useState<Toast[]>([])
  const [overlay, setOverlay] = useState<Overlay>((devFlags.overlay as Overlay) ?? null)
  const [composerDraft, setComposerDraftState] = useState(EMPTY_DRAFT)
  const [composerMode, setComposerModeState] = useState<ComposerMode>('thread')
  const [composerAccess, setComposerAccess] = useState<Access>('supervised')
  const [focusRequest, setFocusRequest] = useState<Orchestrator['focusRequest']>({ target: 'thread', n: 0 })

  const threadTree = useMemo(() => threadTreeOf(sessions), [sessions])
  const sessionsRef = useRef(sessions)
  const sideChatsRef = useRef(sideChats)
  const sideStarting = useRef(false)
  const treeRef = useRef(threadTree)
  const projectsRef = useRef(projects)
  const selectedRef = useRef(selectedId)
  const historyRef = useRef(viewHistory)
  const quiet = useRef(false)
  // Whether the project list is read, and the projects whose thread lists are read, so history forgets only views
  // known to be gone. A step through history counts the owner's moves, so one made while it waits on a project wins.
  const projectsRead = useRef(false)
  const threadsRead = useRef(new Set<string>())
  const moves = useRef(0)
  const projectRef = useRef(project)
  const lastSelected = useRef(new Map<string, string>())
  const toastKey = useRef(0)
  const toastTimers = useRef(new Map<number, number>())
  const projectEpoch = useRef(0)
  const lastCleanupToast = useRef(0)
  const warningToasts = useRef(new Map<Warning['source'], { message: string; key: number }>())
  const failedBrowserIds = useRef(new Set<string>())
  const detached = useRef(false)
  const notesRef = useRef(notes)
  const browserStatesRef = useRef(browserStates)
  const issuesRef = useRef<Issue[]>([])
  sessionsRef.current = sessions
  sideChatsRef.current = sideChats
  treeRef.current = threadTree
  projectsRef.current = projects
  projectRef.current = project
  notesRef.current = notes
  browserStatesRef.current = browserStates

  const moveHistory = useCallback((next: History) => {
    if (next === historyRef.current) return
    historyRef.current = next
    setViewHistory(next)
  }, [])

  const remember = useCallback(
    (view: View) => {
      if (quiet.current) return
      const next = push(historyRef.current, view)
      if (next === historyRef.current) return
      moves.current++
      moveHistory(next)
    },
    [moveHistory],
  )

  // forgetGone drops from history the views whose project, or whose thread in a project whose threads are read, is not
  // among projects and sessions.
  const forgetGone = useCallback(
    (sessions: Session[], projects: Project[]) => {
      const threads = new Map([...threadsRead.current].map((key) => [key, new Set(sessions.filter((s) => s.project === key).map((s) => s.id))]))
      const known: Known = { projects: projectsRead.current ? new Set(projects.map((p) => p.key)) : null, threads }
      moveHistory(prune(historyRef.current, (view) => !goneFrom(known, view)))
    },
    [moveHistory],
  )

  // quietly makes the selections in run without adding them to history.
  const quietly = useCallback((run: () => void) => {
    quiet.current = true
    try {
      run()
    } finally {
      quiet.current = false
    }
  }, [])

  const selectId = useCallback(
    (id: string | null) => {
      selectedRef.current = id
      setSelectedId(id)
      remember(viewOf(id, projectRef.current?.key ?? ''))
      if (!id) return
      detached.current = false
    },
    [remember],
  )

  const detach = useCallback(() => {
    const id = selectedRef.current
    if (!id) return
    detached.current = true
    selectId(null)
  }, [selectId])

  const focusListeners = useRef(new Set<(target: FocusTarget) => void>())
  const focus = useCallback((target: FocusTarget) => {
    setFocusRequest((r) => ({ target, n: r.n + 1 }))
    focusListeners.current.forEach((listener) => listener(target))
  }, [])

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

  const publishNewProject = useCallback(
    async (key: string) => {
      try {
        const repo = await api.publishProject(key)
        pushToast({ tone: 'success', text: 'Published to GitHub', detail: repo })
        const snap = await api.snapshot()
        setProjects(snap.projects)
        if (projectRef.current?.key === key) setProject(snap.projects.find((p) => p.key === key) ?? snap.project)
      } catch (err) {
        pushToast({ tone: 'error', text: 'Could not create the GitHub repository', detail: errorMessage(err) })
      }
    },
    [pushToast],
  )

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
    // all is every project's sessions with list in them, from which a thread's family is read.
    (list: Session[], all: Session[], homeUnlessRemembered = false) => {
      const current = selectedRef.current
      if (detached.current || (current && list.some((s) => s.id === current))) return
      const remembered = list.find((s) => s.id === lastSelected.current.get(projectRef.current?.name ?? ''))
      if (homeUnlessRemembered && !remembered) {
        detached.current = true
        return
      }
      const next = remembered ?? firstThread(list, threadTreeOf(all))
      selectId(next ? next.id : null)
    },
    [selectId],
  )

  const applySideChats = useCallback((projectKey: string | null, items: Session[]) => {
    const before = sideChatsRef.current
    const kept = (s: Session) => projectKey !== null && s.project !== projectKey
    before.filter((s) => !kept(s) && !items.some((x) => x.id === s.id)).forEach((s) => forgetThread(s.id))
    setSideChats((prev) => keepUnchanged(prev, [...prev.filter(kept), ...items], (s) => s.id))
  }, [])

  const applySessions = useCallback(
    (projectKey: string, rows: Session[]) => {
      const { threads: items, sideChats } = splitSideChats(rows)
      applySideChats(projectKey, sideChats)
      goneThreads(sessionsRef.current, projectKey, items).forEach(forgetThread)
      setSessions((prev) => keepUnchanged(prev, [...prev.filter((s) => s.project !== projectKey), ...items], (s) => s.id))
      const all = [...sessionsRef.current.filter((s) => s.project !== projectKey), ...items]
      if (projectKey === projectRef.current?.key) selectAmong(items, all)
      threadsRead.current.add(projectKey)
      forgetGone(all, projectsRef.current)
    },
    [selectAmong, forgetGone, applySideChats],
  )

  const loadDigest = useCallback(async (epoch: number) => {
    setDigestError('')
    try {
      const next = await api.digest()
      if (epoch === projectEpoch.current) setDigest(next)
    } catch (err) {
      if (epoch === projectEpoch.current) setDigestError(errorMessage(err))
    }
  }, [])

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

  // record adds the view the project opens on to history; a step through history shows its own view instead.
  const enterProject = useCallback(
    (whole: Snapshot, epoch: number, homeUnlessRemembered = false, record = true) => {
      const { threads, sideChats } = splitSideChats(whole.sessions)
      const snap = { ...whole, sessions: threads }
      applySideChats(null, sideChats)
      projectEpoch.current = epoch
      if (projectRef.current && selectedRef.current) lastSelected.current.set(projectRef.current.name, selectedRef.current)
      projectRef.current = snap.project
      detached.current = false
      quietly(() => selectId(null))
      setProject(snap.project)
      setProjects(snap.projects)
      setNotes(snap.notes)
      setShells(snap.shells)
      shellsChanged(snap.project.key, snap.shells)
      setMachine(snap.machine)
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
      void loadDigest(epoch)
      setDigestSeen(readStored(digestKey(snap.project.name), 0))
      setSessions((prev) => keepUnchanged(prev, snap.sessions, (s) => s.id))
      quietly(() => selectAmong(snap.sessions.filter((s) => s.project === snap.project.key), snap.sessions, homeUnlessRemembered))
      if (record) remember(viewOf(selectedRef.current, snap.project.key))
      // The snapshot holds the threads of every project.
      projectsRead.current = true
      snap.projects.forEach((p) => threadsRead.current.add(p.key))
      forgetGone(snap.sessions, snap.projects)
      report(() => loadIssues(false))
    },
    [selectAmong, loadIssues, loadDigest, report, selectId, remember, quietly, forgetGone, applySideChats],
  )

  // Each request takes the next epoch before it awaits, so only the latest one enters its project.
  const enterFrom = useCallback(
    async (request: Promise<Snapshot>, homeUnlessRemembered = false, record = true) => {
      const epoch = ++projectEpoch.current
      const snap = await request
      if (epoch === projectEpoch.current) enterProject(snap, epoch, homeUnlessRemembered, record)
      return snap
    },
    [enterProject],
  )

  useEffect(() => {
    report(() => enterFrom(api.snapshot()))
    report(async () => setAwake(await api.awake()))
    report(async () => setMemory(await api.memory()))
    report(async () => {
      const loaded = await api.settings()
      setSettings(loaded)
      setComposerAccess(newThreadAccess(loaded.threadAccess))
    })
    report(async () => {
      const notes = await api.newReleaseNotes()
      setStartNotes(notes)
      setOverlay((current) => overlayAtStart(current, notes))
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
  useEffect(
    () =>
      on(
        'shells',
        ifCurrent((items: Shell[]) => {
          setShells(items)
          shellsChanged(projectRef.current!.key, items)
        }),
      ),
    [],
  )
  useEffect(() => on('term:exit', ({ id }) => setShells((list) => list.filter((x) => x.id !== id))), [])
  useEffect(() => on('notes', ifCurrent(setNotes)), [])
  useEffect(() => on('awake', setAwake), [])
  useEffect(() => on('memory', setMemory), [])
  useEffect(() => on('issues', ifCurrent(setRawIssues)), [])
  useEffect(() => {
    // An update event that comes before the first read is newer than what the read gives.
    let heard = false
    const off = on('update', (state) => {
      heard = true
      setUpdate(state)
    })
    report(async () => {
      const state = await api.updateState()
      if (!heard) setUpdate(state)
    })
    return off
  }, [report])
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
        projectsRead.current = true
        forgetGone(sessionsRef.current, list)
        setProject((p) => {
          const next = list.find((x) => x.name === p?.name)
          return next && !sameJSON(next, p) ? next : p
        })
      }),
    [forgetGone],
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
          // A child that waits is answered from its top-level thread's row, so the toast names that thread and opens it.
          const top = treeRef.current.top.get(id)
          if (state === 'waiting' && top && top.id !== id) {
            pushToast({ tone: state, text: `${top.title} · ${current.title} needs you`, sessionId: top.id })
            return
          }
          pushToast({ tone: state, text: `${current.title} ${describe(current)}`, sessionId: id, view: state === 'evidence' ? 'media' : undefined })
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
    const next = rawIssues.map((issue) => ({ ...issue, sessionId: projectSessions.find((s) => !s.forkOf && s.issue === issue.number)?.id ?? '' }))
    issuesRef.current = keepUnchanged(issuesRef.current, next, (issue) => issue.number)
    return issuesRef.current
  }, [rawIssues, projectSessions])

  const addSession = useCallback(
    (created: Session, background = false) => {
      setSessions((list) => (list.some((s) => s.id === created.id) ? list.map((s) => (s.id === created.id ? created : s)) : [...list, created]))
      if (!background && created.project === projectRef.current?.key) {
        selectId(created.id)
        focus('thread')
      }
      return created
    },
    [selectId, focus],
  )

  const inSidebarOrder = useCallback(() => {
    return sidebarThreads(sessionsRef.current, projectRef.current?.key, treeRef.current)
  }, [])

  const selectSession = useCallback(
    (id: string) => {
      const target = sessionsRef.current.find((s) => s.id === id)
      if (!target) {
        pushToast({ tone: 'info', text: 'That thread is gone' })
        return
      }
      const owner = target.project !== projectRef.current?.key ? projectsRef.current.find((p) => p.key === target.project) : undefined
      if (!owner) {
        selectId(id)
        focus('thread')
        return
      }
      report(async () => {
        await enterFrom(api.switchProject(owner.name), false, false)
        if (projectRef.current?.key !== owner.key) return
        if (!sessionsRef.current.some((s) => s.id === id)) return remember(viewOf(selectedRef.current, owner.key))
        selectId(id)
        focus('thread')
      })
    },
    [selectId, focus, report, enterFrom, remember, pushToast],
  )

  // goThrough steps through history to the first view whose thread and project are still there, entering its project
  // first. History moves only once the view shows, so a project that fails to open leaves it where it was; a view the
  // owner left meanwhile is not shown.
  const goThrough = useCallback(
    (step: (h: History) => History) => {
      const from = historyRef.current
      const to = stepTo(from, step, shownIn(sessionsRef.current, projectsRef.current))
      if (to === from) return
      const view = current(to)!
      const owner = projectsRef.current.find((p) => p.key === projectOf(view))!
      const move = ++moves.current
      // Entering the project can prune the history, so the step lands on the view's entry in the history as it is then.
      const show = () => {
        const at = historyRef.current.views.indexOf(view)
        if (at < 0) return
        moveHistory({ ...historyRef.current, at })
        quietly(() => {
          if (view.kind === 'home') {
            detached.current = true
            selectId(null)
            return
          }
          selectId(threadOf(view))
          focus('thread')
        })
      }
      if (owner.key === projectRef.current?.key) return show()
      report(async () => {
        await enterFrom(api.switchProject(owner.name), false, false)
        if (moves.current === move && projectRef.current?.key === owner.key) show()
      })
    },
    [moveHistory, quietly, selectId, focus, report, enterFrom],
  )

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

  const actions = useMemo<OrchestratorActions>(
    () => ({
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
          await enterFrom(api.switchProject(owner.name), false, false)
          if (projectRef.current?.key === owner.key) show()
        })
      },
      openAgent(thread, agent) {
        if (selectedRef.current !== thread) return
        remember({ kind: 'agent', thread, agent })
      },
      closeGoneAgent(thread, loaded, holds) {
        moveHistory(goneAgent(historyRef.current, thread, loaded, holds))
      },
      goBack: () => goThrough(back),
      goForward: () => goThrough(forward),
      detach,
      stepSession(delta) {
        const list = inSidebarOrder()
        if (list.length === 0) return
        const selected = treeRef.current.top.get(selectedRef.current ?? '')
        const at = list.findIndex((s) => s.id === selected?.id)
        selectSession(list[(at + delta + list.length) % list.length].id)
      },
      async newThread({ prompt, model, effort }, access, place) {
        return addSession(await api.newThread('', prompt, model, effort, access, place))
      },
      async settleThread(id) {
        await api.settleThread(id)
        if (selectedRef.current !== id) return
        const next = nextThread(sessionsRef.current, projectRef.current?.key, id, treeRef.current)
        if (next) selectId(next.id)
        else detach()
      },
      // The delete takes the thread's asleep descendants too. The sessions event may come before the call returns, so
      // the rows go from the latest list, never from one read before the await.
      async deleteThread(id, worktree) {
        const owner = sessionsRef.current.find((s) => s.id === id)?.project
        const gone = new Set([id, ...descendantsOf(id, sessionsRef.current).map((s) => s.id)])
        await api.deleteThread(id, worktree)
        gone.forEach(forgetThread)
        setSessions((prev) => prev.filter((s) => !gone.has(s.id)))
        const left = sessionsRef.current.filter((s) => !gone.has(s.id))
        if (owner === projectRef.current?.key) selectAmong(left.filter((s) => s.project === owner), left)
        forgetGone(left, projectsRef.current)
      },
      async wakeThread(id) {
        return addSession(await api.wakeThread(id))
      },
      async setAccess(id, access) {
        await api.setAccess(id, access)
      },
      async openInEditor(id) {
        await api.openInEditor(id, '')
      },
      async forkSession(id, at, draft = '') {
        const fork = await api.forkSession(id, at)
        if (draft.trim()) writeStored(threadDraftKey(fork.id), saveDraft(draftOf(fromMarkdown(draft))))
        return addSession(fork)
      },
      async startSideChat(parent, text, images) {
        if (sideStarting.current) throw 'Another side chat is still starting. Try again in a moment.'
        sideStarting.current = true
        try {
          const side = await api.sideChatStart(parent, text, images.map(({ base64, mime }) => ({ base64, mime })))
          setSideChats((list) => [...list.filter((s) => s.id !== side.id), side])
          return side
        } finally {
          sideStarting.current = false
        }
      },
      async deleteSideChat(id, unused) {
        await api.sideChatDelete(id, unused)
        if (unused) return
        forgetThread(id)
        setSideChats((list) => list.filter((s) => s.id !== id))
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
      async addProject(dir, create = false) {
        await enterFrom(api.addProjectDir(dir, create))
      },
      async createProject(name, publish) {
        let created: NewProject
        try {
          created = await api.createProject(name)
        } catch (err) {
          pushToast({ tone: 'error', text: 'Could not create the project', detail: errorMessage(err) })
          return false
        }
        pushToast(
          created.commitError
            ? { tone: 'warning', text: `Created ${name} without a first commit`, detail: `${created.commitError} The project is in ${created.dir}.` }
            : { tone: 'success', text: `Created ${name}`, detail: created.dir },
        )
        const key = created.snapshot.project.key
        if (publish) void publishNewProject(key)
        await enterFrom(Promise.resolve(created.snapshot))
        focus('composer')
        return true
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
      refreshIssues: () => loadIssues(true),
      loadBrowserState,
      async openBrowser(id, url) {
        const state = browserStatesRef.current[id] ?? (await api.browserState(id))
        if (!state.open) {
          const opened = await api.browserOpen(id, url ?? '')
          setBrowserStates((map) => ({ ...map, [id]: opened }))
        } else if (url) {
          await api.browserGoto(id, url)
        }
      },
      reloadDigest() {
        void loadDigest(projectEpoch.current)
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
        const urgent = urgentThreads(projectsRef.current, sessionsRef.current, treeRef.current)
        if (urgent.length === 0) throw 'Nothing needs you right now'
        const selected = treeRef.current.top.get(selectedRef.current ?? '')
        const at = urgent.findIndex((s) => s.id === selected?.id)
        selectSession(urgent[(at + 1) % urgent.length].id)
      },
      setComposerDraft(draft, project = projectRef.current?.name) {
        if (!project) return
        if (project === projectRef.current?.name) setComposerDraftState(draft)
        writeStored(draftKey(project), saveDraft(draft))
      },
      threadDraft(id) {
        return { draft: readDraft(readStored<unknown>(threadDraftKey(id), '')), seen: readStored(draftSeenKey(id), 0) }
      },
      setThreadDraft(id, draft) {
        if (draft.text.trim() === '') removeStored(threadDraftKey(id))
        else writeStored(threadDraftKey(id), saveDraft(draft))
      },
      setDraftsSeen(id, seq) {
        writeStored(draftSeenKey(id), seq)
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
      async setProjectsDir(dir) {
        setSettings(await api.setProjectsDir(dir))
      },
      async setKeepAwake(on) {
        setSettings(await api.setKeepAwake(on))
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
      async setThreadPlace(place) {
        setSettings(await api.setThreadPlace(place))
      },
      async setThreadAccess(access) {
        const saved = await api.setThreadAccess(access)
        setSettings(saved)
        setComposerAccess(newThreadAccess(saved.threadAccess))
      },
      setComposerMode(mode) {
        setComposerModeState(mode)
        if (projectRef.current) writeStored(composerModeKey(projectRef.current.name), mode)
      },
      setComposerAccess,
      setIssueFilter(query) {
        setIssueFilterState(query)
        if (projectRef.current) writeStored(filterKey(projectRef.current.name), query)
      },
      focus,
      onFocus(listener) {
        focusListeners.current.add(listener)
        return () => focusListeners.current.delete(listener)
      },
      report,
      pushToast,
      dismissToast,
    }),
    [selectId, selectSession, remember, moveHistory, goThrough, inSidebarOrder, detach, addSession, selectAmong, enterProject, enterFrom, loadIssues, loadDigest, loadBrowserState, markDigestSeen, answerFailed, focus, report, pushToast, dismissToast, publishNewProject],
  )

  const live = shownIn(sessions, projects)
  const canGoBack = stepTo(viewHistory, back, live) !== viewHistory
  const canGoForward = stepTo(viewHistory, forward, live) !== viewHistory

  const value = useMemo<Orchestrator>(
    () => ({
      project,
      projects,
      sessions,
      sideChats,
      projectSessions,
      threadTree,
      selectedId,
      viewHistory,
      canGoBack,
      canGoForward,
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
      digestError,
      digestSeen,
      digestUnseenCount,
      awake,
      memory,
      settings,
      toasts,
      overlay,
      composerDraft,
      composerMode,
      composerAccess,
      focusRequest,
      shells,
      machine,
      update,
      startNotes,
      ...actions,
    }),
    [project, projects, sessions, sideChats, projectSessions, threadTree, selectedId, viewHistory, canGoBack, canGoForward, issues, issuesLoading, issuesDisabled, issueFilter, notes, cleanups, evidence, browserStates, pending, digest, digestError, digestSeen, digestUnseenCount, awake, memory, settings, toasts, overlay, composerDraft, composerMode, composerAccess, focusRequest, shells, machine, update, startNotes, actions],
  )

  const [store] = useState(() => createStore(value))
  useLayoutEffect(() => store.set(value), [store, value])

  return <OrchestratorContext.Provider value={store}>{children}</OrchestratorContext.Provider>
}
