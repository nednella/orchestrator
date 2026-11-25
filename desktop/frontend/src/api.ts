import { chatMediaURL } from './chatEvents'
import type { Command } from './chatTimeline'
import { createMock } from './mock'
import type { Access, BrowserState, ChatEvent, ChatImage, ChatSent, Cleanup, CleanupEvent, CleanupMode, DarkTheme, DataDirChoice, Digest, DigestSchedule, EventMap, Evidence, Issue, IssueDetail, Ledger, LightTheme, Memory, ModelChoice, Note, Pending, Place, Release, Session, PromptSend, Settings, Snapshot, Stats, ThemeSetting, WaitKind } from './types'

type Backend = {
  Snapshot(): Promise<Snapshot>
  NewThread(title: string, prompt: string, model: string, effort: string, access: Access, place: Place | ''): Promise<Session>
  SettleThread(id: string): Promise<void>
  ArchiveThread(id: string): Promise<void>
  WakeThread(id: string): Promise<Session>
  SetAccess(id: string, access: Access): Promise<void>
  ForkSession(id: string, at: string): Promise<Session>
  RenameSession(id: string, title: string): Promise<void>
  SwitchProject(name: string): Promise<Snapshot>
  AddProject(): Promise<Snapshot>
  AddProjectDir(dir: string): Promise<Snapshot>
  NewProject(name: string): Promise<Snapshot>
  CreateRepo(name: string): Promise<Snapshot>
  RemoveProject(name: string): Promise<Snapshot>
  SetUpProject(): Promise<Session>
  DismissSetup(): Promise<Snapshot>
  AddNote(text: string): Promise<Note>
  UpdateNote(id: string, text: string): Promise<Note>
  SetNotePinned(id: string, pinned: boolean): Promise<Note>
  SetNoteArchived(id: string, archived: boolean): Promise<Note>
  AddNoteImage(id: string, base64: string, mime: string): Promise<Note>
  RemoveNoteImage(id: string, url: string): Promise<Note>
  DeleteNote(id: string): Promise<void>
  NoteToIssue(id: string): Promise<void>
  NoteToSession(id: string): Promise<Session>
  Issues(refresh: boolean): Promise<Issue[]>
  StartIssue(number: number, action: string, prompt: string, model: string, effort: string): Promise<Session>
  MoveIssue(number: number, section: string): Promise<void>
  UndoMove(number: number): Promise<void>
  IssueDetail(number: number): Promise<IssueDetail>
  ShellOpen(sessionId: string): Promise<{ id: string }>
  ShellNew(sessionId: string): Promise<{ id: string }>
  ShellClose(id: string): Promise<void>
  TermOpen(id: string, cols: number, rows: number): Promise<void>
  TermWrite(id: string, data: string): Promise<void>
  TermResize(id: string, cols: number, rows: number): Promise<void>
  TermClose(id: string): Promise<void>
  OpenURL(url: string): Promise<void>
  Stats(days: number, project: string): Promise<Stats>
  Ledger(days: number, project: string): Promise<Ledger>
  RefreshPRs(): Promise<void>
  AckPR(id: string): Promise<void>
  TypeInto(id: string, text: string): Promise<void>
  ChatHistory(id: string, after: number): Promise<ChatEvent[]>
  ChatWatch(id: string): Promise<void>
  ChatUnwatch(id: string): Promise<void>
  ChatSend(id: string, text: string, images: ChatImage[]): Promise<ChatSent>
  ChatDraft(id: string, text: string): Promise<void>
  ChatInterrupt(id: string): Promise<void>
  ChatSendNow(id: string, message: string): Promise<void>
  ChatAnswer(id: string, request: string, allow: boolean, message: string): Promise<void>
  ChatAnswerInput(id: string, request: string, input: string): Promise<void>
  ChatPending(): Promise<Pending[]>
  ChatFiles(id: string, query: string): Promise<string[]>
  ProjectFiles(project: string, query: string): Promise<string[]>
  ProjectCommands(project: string): Promise<Command[]>
  Cleanup(id: string, force: boolean): Promise<void>
  Cleanups(): Promise<Cleanup[]>
  Memory(): Promise<Memory>
  DefaultModel(model: string): Promise<ModelChoice>
  BrowserOpen(id: string, url: string): Promise<BrowserState>
  BrowserGoto(id: string, url: string): Promise<void>
  BrowserShow(id: string): Promise<void>
  BrowserState(id: string): Promise<BrowserState>
  BrowserScreenshot(id: string, caption: string): Promise<Evidence>
  BrowserClose(id: string): Promise<void>
  Evidence(id: string): Promise<Evidence[]>
  DeleteEvidence(id: string, evidenceId: string): Promise<void>
  Digest(): Promise<Digest>
  RunDigest(): Promise<void>
  DigestToNote(itemId: string): Promise<Note>
  DismissDigestItem(itemId: string): Promise<void>
  Update(): Promise<void>
  PatchNotes(): Promise<Release[]>
  Changelog(): Promise<Release[]>
  Awake(): Promise<boolean>
  Settings(): Promise<Settings>
  SetTheme(theme: ThemeSetting): Promise<Settings>
  SetThemes(dark: DarkTheme, light: LightTheme): Promise<Settings>
  SetCleanup(event: CleanupEvent, mode: CleanupMode): Promise<Settings>
  SetTextScale(scale: number): Promise<Settings>
  SetKeepAwake(on: boolean): Promise<Settings>
  SetPromptSend(mode: PromptSend): Promise<Settings>
  SetBrowserEnabled(on: boolean): Promise<Settings>
  SetDigestSchedule(schedule: DigestSchedule): Promise<Settings>
  PickDataDir(): Promise<DataDirChoice>
  SetDataDir(dir: string, withData: boolean): Promise<void>
}

type Unsubscribe = () => void

type Runtime = {
  BrowserOpenURL?(url: string): void
  WindowIsFullscreen?(): Promise<boolean>
  EventsOn(name: string, handler: (payload: never) => void): Unsubscribe | void
}

declare global {
  interface Window {
    go?: Record<string, { Service: Record<string, (...args: unknown[]) => Promise<unknown>> }>
    runtime?: Runtime
  }
}

// The Go side binds one service per package; window.go.<namespace>.Service.<Method>.
const namespaces = {
  projects: ['Snapshot', 'SwitchProject', 'AddProject', 'AddProjectDir', 'NewProject', 'CreateRepo', 'RemoveProject', 'SetUpProject', 'DismissSetup'],
  sessions: ['NewThread', 'SettleThread', 'ArchiveThread', 'WakeThread', 'SetAccess', 'ForkSession', 'RenameSession', 'TypeInto', 'RefreshPRs', 'AckPR', 'Cleanup', 'Cleanups', 'ShellOpen', 'ShellNew', 'ShellClose', 'Memory', 'DefaultModel'],
  chat: ['ChatHistory', 'ChatWatch', 'ChatUnwatch', 'ChatSend', 'ChatDraft', 'ChatInterrupt', 'ChatSendNow', 'ChatAnswer', 'ChatAnswerInput', 'ChatPending', 'ChatFiles', 'ProjectFiles', 'ProjectCommands'],
  terminal: ['TermOpen', 'TermWrite', 'TermResize', 'TermClose'],
  issues: ['Issues', 'StartIssue', 'MoveIssue', 'UndoMove', 'IssueDetail'],
  notes: ['AddNote', 'UpdateNote', 'SetNotePinned', 'SetNoteArchived', 'AddNoteImage', 'RemoveNoteImage', 'DeleteNote', 'NoteToIssue', 'NoteToSession'],
  stats: ['Stats', 'Ledger'],
  evidence: ['Evidence', 'DeleteEvidence'],
  browser: ['BrowserOpen', 'BrowserGoto', 'BrowserShow', 'BrowserState', 'BrowserScreenshot', 'BrowserClose'],
  digest: ['Digest', 'RunDigest', 'DigestToNote', 'DismissDigestItem'],
  update: ['Update', 'PatchNotes', 'Changelog'],
  awake: ['Awake'],
  settings: ['Settings', 'SetTheme', 'SetThemes', 'SetCleanup', 'SetTextScale', 'SetKeepAwake', 'SetPromptSend', 'SetBrowserEnabled', 'SetDigestSchedule', 'PickDataDir', 'SetDataDir'],
}

// The Go side has no method for this: the window runtime opens web addresses itself.
async function openInBrowser(url: string) {
  if (!/^https?:\/\//.test(url)) throw `Not a web address: ${url}`
  if (window.runtime?.BrowserOpenURL) window.runtime.BrowserOpenURL(url)
  else window.open(url, '_blank')
}

function bindServices(go: NonNullable<Window['go']>): Backend {
  const methods = Object.entries(namespaces).flatMap(([namespace, names]) =>
    names.map((name) => [name, (...args: unknown[]) => go[namespace].Service[name](...args)]),
  )
  return { ...Object.fromEntries(methods), OpenURL: openInBrowser } as Backend
}

export type DevFlags = { overlay?: string; tab?: string; cmd?: string; view?: string }

const mock = window.go ? null : createMock(new URLSearchParams(location.search))
const backend: Backend = window.go ? bindServices(window.go) : mock!.backend

export const devFlags: DevFlags = mock?.flags ?? {}

export const api = {
  snapshot: () => backend.Snapshot(),
  newThread: (title: string, prompt = '', model = '', effort = '', access: Access = 'supervised', place: Place | '' = '') => backend.NewThread(title, prompt, model, effort, access, place),
  settleThread: (id: string) => backend.SettleThread(id),
  archiveThread: (id: string) => backend.ArchiveThread(id),
  wakeThread: (id: string) => backend.WakeThread(id),
  setAccess: (id: string, access: Access) => backend.SetAccess(id, access),
  forkSession: (id: string, at: string) => backend.ForkSession(id, at),
  renameSession: (id: string, title: string) => backend.RenameSession(id, title),
  switchProject: (name: string) => backend.SwitchProject(name),
  addProject: () => backend.AddProject(),
  addProjectDir: (dir: string) => backend.AddProjectDir(dir),
  newProject: (name: string) => backend.NewProject(name),
  createRepo: (name: string) => backend.CreateRepo(name),
  removeProject: (name: string) => backend.RemoveProject(name),
  setUpProject: () => backend.SetUpProject(),
  dismissSetup: () => backend.DismissSetup(),
  addNote: (text: string) => backend.AddNote(text),
  updateNote: (id: string, text: string) => backend.UpdateNote(id, text),
  setNotePinned: (id: string, pinned: boolean) => backend.SetNotePinned(id, pinned),
  setNoteArchived: (id: string, archived: boolean) => backend.SetNoteArchived(id, archived),
  addNoteImage: (id: string, base64: string, mime: string) => backend.AddNoteImage(id, base64, mime),
  removeNoteImage: (id: string, url: string) => backend.RemoveNoteImage(id, url),
  deleteNote: (id: string) => backend.DeleteNote(id),
  noteToIssue: (id: string) => backend.NoteToIssue(id),
  noteToSession: (id: string) => backend.NoteToSession(id),
  issues: (refresh: boolean) => backend.Issues(refresh),
  startIssue: (number: number, action: string, prompt = '', model = '', effort = '') => backend.StartIssue(number, action, prompt, model, effort),
  moveIssue: (number: number, section: string) => backend.MoveIssue(number, section),
  undoMove: (number: number) => backend.UndoMove(number),
  issueDetail: (number: number) => backend.IssueDetail(number),
  shellOpen: (sessionId: string) => backend.ShellOpen(sessionId),
  shellNew: (sessionId: string) => backend.ShellNew(sessionId),
  shellClose: (id: string) => backend.ShellClose(id),
  termOpen: (id: string, cols: number, rows: number) => backend.TermOpen(id, cols, rows),
  // A terminal can close (project switch, session end) while keys or a resize
  // are still in flight; that is not an error worth surfacing.
  termWrite: (id: string, data: string) => backend.TermWrite(id, data).catch(() => {}),
  termResize: (id: string, cols: number, rows: number) => backend.TermResize(id, cols, rows).catch(() => {}),
  termClose: (id: string) => backend.TermClose(id),
  openURL: (url: string) => backend.OpenURL(url),
  stats: (days: number, project: string) => backend.Stats(days, project).then(readIdleKinds),
  ledger: (days: number, project: string) => backend.Ledger(days, project),
  refreshPRs: () => backend.RefreshPRs(),
  ackPR: (id: string) => backend.AckPR(id),
  typeInto: (id: string, text: string) => backend.TypeInto(id, text),
  // Watch first, then read the history, and merge the two by seq: an event logged in between comes in both.
  chatHistory: (id: string, after = 0) => backend.ChatHistory(id, after),
  chatWatch: (id: string) => backend.ChatWatch(id),
  chatUnwatch: (id: string) => backend.ChatUnwatch(id),
  chatSend: (id: string, text: string, images: ChatImage[] = []) => backend.ChatSend(id, text, images),
  // The mock keeps the images it was sent in memory and has its own address for each.
  chatMediaURL: (thread: string, path: string) => mock?.mediaURL(path) ?? chatMediaURL(thread, path),
  chatDraft: (id: string, text: string) => backend.ChatDraft(id, text),
  chatInterrupt: (id: string) => backend.ChatInterrupt(id),
  chatSendNow: (id: string, message: string) => backend.ChatSendNow(id, message),
  chatAnswer: (id: string, request: string, allow: boolean, message = '') => backend.ChatAnswer(id, request, allow, message),
  chatAnswerInput: (id: string, request: string, input: string) => backend.ChatAnswerInput(id, request, input),
  chatPending: () => backend.ChatPending(),
  chatFiles: (id: string, query: string) => backend.ChatFiles(id, query),
  projectFiles: (project: string, query: string) => backend.ProjectFiles(project, query),
  projectCommands: (project: string) => backend.ProjectCommands(project),
  cleanup: (id: string, force: boolean) => backend.Cleanup(id, force),
  cleanups: () => backend.Cleanups(),
  memory: () => backend.Memory(),
  defaultModel: (model = '') => backend.DefaultModel(model),
  browserOpen: (id: string, url: string) => backend.BrowserOpen(id, url),
  browserGoto: (id: string, url: string) => backend.BrowserGoto(id, url),
  browserShow: (id: string) => backend.BrowserShow(id),
  browserState: (id: string) => backend.BrowserState(id),
  browserScreenshot: (id: string, caption: string) => backend.BrowserScreenshot(id, caption),
  browserClose: (id: string) => backend.BrowserClose(id),
  evidence: (id: string) => backend.Evidence(id),
  deleteEvidence: (id: string, evidenceId: string) => backend.DeleteEvidence(id, evidenceId),
  awake: () => backend.Awake(),
  settings: () => backend.Settings(),
  setTheme: (theme: ThemeSetting) => backend.SetTheme(theme),
  setThemes: (dark: DarkTheme, light: LightTheme) => backend.SetThemes(dark, light),
  setCleanup: (event: CleanupEvent, mode: CleanupMode) => backend.SetCleanup(event, mode),
  setTextScale: (scale: number) => backend.SetTextScale(scale),
  setKeepAwake: (on: boolean) => backend.SetKeepAwake(on),
  setPromptSend: (mode: PromptSend) => backend.SetPromptSend(mode),
  setBrowserEnabled: (on: boolean) => backend.SetBrowserEnabled(on),
  setDigestSchedule: (schedule: DigestSchedule) => backend.SetDigestSchedule(schedule),
  pickDataDir: () => backend.PickDataDir(),
  setDataDir: (dir: string, withData: boolean) => backend.SetDataDir(dir, withData),
  digest: () => backend.Digest(),
  runDigest: () => backend.RunDigest(),
  digestToNote: (itemId: string) => backend.DigestToNote(itemId),
  dismissDigestItem: (itemId: string) => backend.DismissDigestItem(itemId),
  update: () => backend.Update(),
  patchNotes: () => backend.PatchNotes(),
  changelog: () => backend.Changelog(),
}

// The Go side used to call an idle wait "finished"; accept both.
type LegacyWaitKind = WaitKind | 'finished'

function readIdleKinds(stats: Stats): Stats {
  const idle = (kind: LegacyWaitKind): WaitKind => (kind === 'finished' ? 'idle' : kind)
  return {
    ...stats,
    byCause: stats.byCause.map((c) => ({ ...c, kind: idle(c.kind) })),
    recent: stats.recent.map((w) => ({ ...w, kind: idle(w.kind) })),
  }
}

export function on<K extends keyof EventMap>(
  event: K,
  handler: (payload: EventMap[K]) => void,
): Unsubscribe {
  if (mock) return mock.on(event, handler)
  const off = window.runtime?.EventsOn(event, handler as (payload: never) => void)
  return off ?? (() => undefined)
}

// chatEvent is the name of the thread's chat:<id> event.
export const chatEvent = (id: string) => `chat:${id}` as `chat:${string}/${string}`

// What `Issues` rejects with when the repo has issues turned off (desktop/issues.ErrIssuesDisabled).
export const ISSUES_DISABLED = 'issues are disabled for this repo'

export function errorMessage(err: unknown): string {
  if (typeof err === 'string') return err
  if (err instanceof Error) return err.message
  return 'Something went wrong'
}
