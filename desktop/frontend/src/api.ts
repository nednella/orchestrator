import { chatMediaURL } from './chatEvents'
import type { Command } from './chatTimeline'
import { createMock } from './mock'
import { openTermStream } from './termStream'
import type { Access, BrowserState, ChatEvent, ChatImage, ChatSent, Cleanup, CleanupEvent, CleanupMode, DarkTheme, DeletePlan, DataDirChoice, Digest, DigestSchedule, EventMap, Evidence, FilesRewind, FolderListing, Issue, IssueDetail, FileContent, FileListing, Ledger, LightTheme, Limits, Memory, ModelChoice, NewProject, NewProjectOptions, Note, Pending, Place, Release, RepoPulls, Server, Session, PromptSend, Settings, Snapshot, ThemeSetting, UpdateState } from './types'

type Backend = {
  Snapshot(): Promise<Snapshot>
  NewThread(title: string, prompt: string, model: string, effort: string, access: Access, place: Place | ''): Promise<Session>
  SettleThread(id: string): Promise<void>
  DeleteThread(id: string, worktree: boolean): Promise<void>
  DeletePlan(id: string): Promise<DeletePlan>
  Servers(id: string): Promise<Server[]>
  PullRequests(): Promise<RepoPulls>
  Viewer(): Promise<string>
  StopServer(id: string, pid: number): Promise<void>
  OpenServer(port: number): Promise<void>
  WakeThread(id: string): Promise<Session>
  SetAccess(id: string, access: Access): Promise<void>
  OpenInEditor(id: string, path: string): Promise<void>
  OpenProjectInEditor(project: string, path: string): Promise<void>
  ForkSession(id: string, at: string): Promise<Session>
  SideChatDelete(id: string, unused: boolean): Promise<void>
  RenameSession(id: string, title: string): Promise<void>
  SwitchProject(name: string): Promise<Snapshot>
  PickFolder(): Promise<string>
  AddProjectDir(dir: string, create: boolean): Promise<Snapshot>
  NewProjectOptions(): Promise<NewProjectOptions>
  GitHubOwner(): Promise<string>
  ListFolders(path: string): Promise<FolderListing>
  CreateProject(name: string): Promise<NewProject>
  PublishProject(key: string): Promise<string>
  SetUpProject(): Promise<Session>
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
  ShellNew(thread: string): Promise<{ id: string }>
  ShellClose(id: string): Promise<void>
  TermOpen(id: string, cols: number, rows: number): Promise<void>
  TermWrite(id: string, data: string): Promise<void>
  TermResize(id: string, cols: number, rows: number): Promise<void>
  TermClose(id: string): Promise<void>
  OpenURL(url: string): Promise<void>
  Ledger(days: number, project: string): Promise<Ledger>
  RefreshPRs(): Promise<void>
  AckPR(id: string): Promise<void>
  TypeInto(id: string, text: string): Promise<void>
  ChatHistory(id: string, after: number): Promise<ChatEvent[]>
  ChatWatch(id: string): Promise<void>
  ChatUnwatch(id: string): Promise<void>
  ChatSend(id: string, text: string, images: ChatImage[], sideChats: string[]): Promise<ChatSent>
  SideChatStart(parent: string, text: string, images: ChatImage[]): Promise<Session>
  ChatInterrupt(id: string): Promise<void>
  ChatSendNow(id: string, message: string): Promise<void>
  ChatAnswer(id: string, request: string, allow: boolean, message: string): Promise<void>
  ChatAnswerInput(id: string, request: string, input: string): Promise<void>
  ChatPending(): Promise<Pending[]>
  ChatFiles(id: string, query: string): Promise<string[]>
  RewindFilesOffer(id: string): Promise<FilesRewind>
  RewindPreview(id: string, message: string): Promise<FilesRewind>
  RewindTo(id: string, message: string, files: boolean): Promise<string>
  RewindOff(): Promise<string>
  ProjectFiles(project: string, query: string): Promise<string[]>
  ChatReadFile(id: string, path: string): Promise<FileContent>
  ProjectReadFile(project: string, path: string): Promise<FileContent>
  ChatFileTree(id: string): Promise<FileListing>
  ProjectFileTree(project: string): Promise<FileListing>
  ChatFileDir(id: string, dir: string): Promise<FileListing>
  ProjectFileDir(project: string, dir: string): Promise<FileListing>
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
  UpdateState(): Promise<UpdateState>
  CheckUpdate(): Promise<void>
  DownloadUpdate(): Promise<void>
  InstallUpdate(): Promise<void>
  NewReleaseNotes(): Promise<Release[]>
  ReleaseNotes(): Promise<Release>
  Limits(): Promise<Limits[]>
  RefreshLimits(): Promise<Limits[]>
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
  SetThreadPlace(place: Place | ''): Promise<Settings>
  SetThreadAccess(access: Access | ''): Promise<Settings>
  SetProjectsDir(dir: string): Promise<Settings>
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
  projects: ['Snapshot', 'SwitchProject', 'PickFolder', 'AddProjectDir', 'NewProjectOptions', 'GitHubOwner', 'ListFolders', 'CreateProject', 'PublishProject', 'SetUpProject'],
  sessions: ['NewThread', 'SettleThread', 'DeleteThread', 'DeletePlan', 'Servers', 'PullRequests', 'Viewer', 'StopServer', 'OpenServer', 'WakeThread', 'SetAccess', 'OpenInEditor', 'OpenProjectInEditor', 'ForkSession', 'SideChatDelete', 'RenameSession', 'TypeInto', 'RefreshPRs', 'AckPR', 'Cleanup', 'Cleanups', 'ShellNew', 'ShellClose', 'Memory', 'DefaultModel'],
  chat: ['ChatHistory', 'ChatWatch', 'ChatUnwatch', 'ChatSend', 'SideChatStart', 'ChatInterrupt', 'ChatSendNow', 'ChatAnswer', 'ChatAnswerInput', 'ChatPending', 'ChatFiles', 'RewindFilesOffer', 'RewindPreview', 'RewindTo', 'RewindOff', 'ProjectFiles', 'ProjectCommands', 'ChatReadFile', 'ProjectReadFile', 'ChatFileTree', 'ProjectFileTree', 'ChatFileDir', 'ProjectFileDir'],
  terminal: ['TermOpen', 'TermWrite', 'TermResize', 'TermClose'],
  issues: ['Issues', 'StartIssue', 'MoveIssue', 'UndoMove', 'IssueDetail'],
  notes: ['AddNote', 'UpdateNote', 'SetNotePinned', 'SetNoteArchived', 'AddNoteImage', 'RemoveNoteImage', 'DeleteNote', 'NoteToIssue', 'NoteToSession'],
  stats: ['Ledger'],
  evidence: ['Evidence', 'DeleteEvidence'],
  browser: ['BrowserOpen', 'BrowserGoto', 'BrowserShow', 'BrowserState', 'BrowserScreenshot', 'BrowserClose'],
  digest: ['Digest', 'RunDigest', 'DigestToNote', 'DismissDigestItem'],
  update: ['UpdateState', 'CheckUpdate', 'DownloadUpdate', 'InstallUpdate', 'NewReleaseNotes', 'ReleaseNotes'],
  limits: ['Limits', 'RefreshLimits'],
  awake: ['Awake'],
  settings: ['Settings', 'SetTheme', 'SetThemes', 'SetCleanup', 'SetTextScale', 'SetKeepAwake', 'SetPromptSend', 'SetBrowserEnabled', 'SetDigestSchedule', 'SetThreadPlace', 'SetThreadAccess', 'SetProjectsDir', 'PickDataDir', 'SetDataDir'],
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

export type DevFlags = { overlay?: string }

const mock = window.go ? null : createMock(new URLSearchParams(location.search))
const backend: Backend = window.go ? bindServices(window.go) : mock!.backend

export const devFlags: DevFlags = mock?.flags ?? {}

export const api = {
  snapshot: () => backend.Snapshot(),
  newThread: (title: string, prompt = '', model = '', effort = '', access: Access = 'supervised', place: Place | '' = '') => backend.NewThread(title, prompt, model, effort, access, place),
  settleThread: (id: string) => backend.SettleThread(id),
  deleteThread: (id: string, worktree: boolean) => backend.DeleteThread(id, worktree),
  deletePlan: (id: string) => backend.DeletePlan(id),
  servers: (id: string) => backend.Servers(id),
  pullRequests: () => backend.PullRequests(),
  viewer: () => backend.Viewer(),
  // The Go side stops pid only once it finds it among the thread's servers again.
  stopServer: (id: string, pid: number) => backend.StopServer(id, pid),
  openServer: (port: number) => backend.OpenServer(port),
  wakeThread: (id: string) => backend.WakeThread(id),
  setAccess: (id: string, access: Access) => backend.SetAccess(id, access),
  // A path opens that markdown file in the thread's folder, '' the folder itself.
  openInEditor: (id: string, path: string) => backend.OpenInEditor(id, path),
  openProjectInEditor: (project: string, path: string) => backend.OpenProjectInEditor(project, path),
  forkSession: (id: string, at: string) => backend.ForkSession(id, at),
  // With unused the side chat goes only when nobody sent it a message, as when its tab closes.
  sideChatDelete: (id: string, unused = false) => backend.SideChatDelete(id, unused),
  renameSession: (id: string, title: string) => backend.RenameSession(id, title),
  switchProject: (name: string) => backend.SwitchProject(name),
  // pickFolder is the native folder picker; '' when the user cancels it.
  pickFolder: () => backend.PickFolder(),
  addProjectDir: (dir: string, create = false) => backend.AddProjectDir(dir, create),
  newProjectOptions: () => backend.NewProjectOptions(),
  // githubOwner is the login gh is signed in as, '' when it is not.
  githubOwner: () => backend.GitHubOwner(),
  listFolders: (path: string) => backend.ListFolders(path),
  createProject: (name: string) => backend.CreateProject(name),
  // publishProject makes the private GitHub repository of the project with the key and gives its owner/name.
  publishProject: (key: string) => backend.PublishProject(key),
  setUpProject: () => backend.SetUpProject(),
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
  shellNew: (thread: string) => backend.ShellNew(thread),
  shellClose: (id: string) => backend.ShellClose(id),
  termOpen: (id: string, cols: number, rows: number) => backend.TermOpen(id, cols, rows),
  // A shell's output is no event: it comes as raw bytes on a stream of its own, readable once termOpen returns.
  termStream: (id: string, onData: (bytes: Uint8Array) => void, onEnd: () => void) => (mock ? mock.termStream(id, onData, onEnd) : openTermStream(id, onData, onEnd)),
  // A terminal can close (project switch, session end) while keys or a resize
  // are still in flight; that is not an error worth surfacing.
  termWrite: (id: string, data: string) => backend.TermWrite(id, data).catch(() => {}),
  termResize: (id: string, cols: number, rows: number) => backend.TermResize(id, cols, rows).catch(() => {}),
  termClose: (id: string) => backend.TermClose(id),
  openURL: (url: string) => backend.OpenURL(url),
  ledger: (days: number, project: string) => backend.Ledger(days, project),
  refreshPRs: () => backend.RefreshPRs(),
  ackPR: (id: string) => backend.AckPR(id),
  typeInto: (id: string, text: string) => backend.TypeInto(id, text),
  // Watch first, then read the history, and merge the two by seq: an event logged in between comes in both.
  chatHistory: (id: string, after = 0) => backend.ChatHistory(id, after),
  chatWatch: (id: string) => backend.ChatWatch(id),
  chatUnwatch: (id: string) => backend.ChatUnwatch(id),
  // Each of sideChats, side chats of the thread, goes with the message as a block of its questions and answers.
  chatSend: (id: string, text: string, images: ChatImage[] = [], sideChats: string[] = []) => backend.ChatSend(id, text, images, sideChats),
  // A side chat forks the thread's conversation at its latest finished reply; text and images, if any, are its first message.
  sideChatStart: (parent: string, text = '', images: ChatImage[] = []) => backend.SideChatStart(parent, text, images),
  // The mock keeps the images it was sent in memory and has its own address for each.
  chatMediaURL: (thread: string, path: string) => mock?.mediaURL(path) ?? chatMediaURL(thread, path),
  chatInterrupt: (id: string) => backend.ChatInterrupt(id),
  chatSendNow: (id: string, message: string) => backend.ChatSendNow(id, message),
  chatAnswer: (id: string, request: string, allow: boolean, message = '') => backend.ChatAnswer(id, request, allow, message),
  chatAnswerInput: (id: string, request: string, input: string) => backend.ChatAnswerInput(id, request, input),
  chatPending: () => backend.ChatPending(),
  chatFiles: (id: string, query: string) => backend.ChatFiles(id, query),
  rewindFilesOffer: (id: string) => backend.RewindFilesOffer(id),
  rewindPreview: (id: string, message: string) => backend.RewindPreview(id, message),
  rewindTo: (id: string, message: string, files: boolean) => backend.RewindTo(id, message, files),
  rewindOff: () => backend.RewindOff(),
  projectFiles: (project: string, query: string) => backend.ProjectFiles(project, query),
  projectCommands: (project: string) => backend.ProjectCommands(project),
  // The Go side reads only a regular file inside the folder, symlinks resolved: the first 1 MB of text, an image of at most 8 MB.
  chatReadFile: (id: string, path: string) => backend.ChatReadFile(id, path),
  projectReadFile: (project: string, path: string) => backend.ProjectReadFile(project, path),
  chatFileTree: (id: string) => backend.ChatFileTree(id),
  projectFileTree: (project: string) => backend.ProjectFileTree(project),
  chatFileDir: (id: string, dir: string) => backend.ChatFileDir(id, dir),
  projectFileDir: (project: string, dir: string) => backend.ProjectFileDir(project, dir),
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
  setThreadPlace: (place: Place | '') => backend.SetThreadPlace(place),
  setThreadAccess: (access: Access | '') => backend.SetThreadAccess(access),
  setProjectsDir: (dir: string) => backend.SetProjectsDir(dir),
  pickDataDir: () => backend.PickDataDir(),
  setDataDir: (dir: string, withData: boolean) => backend.SetDataDir(dir, withData),
  digest: () => backend.Digest(),
  runDigest: () => backend.RunDigest(),
  digestToNote: (itemId: string) => backend.DigestToNote(itemId),
  dismissDigestItem: (itemId: string) => backend.DismissDigestItem(itemId),
  updateState: () => backend.UpdateState(),
  checkUpdate: () => backend.CheckUpdate(),
  downloadUpdate: () => backend.DownloadUpdate(),
  installUpdate: () => backend.InstallUpdate(),
  newReleaseNotes: () => backend.NewReleaseNotes(),
  releaseNotes: () => backend.ReleaseNotes(),
  limits: () => backend.Limits(),
  // Reads the limits now: from a live thread, else by a short probe when the values are over 5 minutes old.
  refreshLimits: () => backend.RefreshLimits(),
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
