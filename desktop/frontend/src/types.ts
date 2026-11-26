export type State = 'waiting' | 'working' | 'idle' | 'settled'

// Access is what a thread may do without asking the owner.
export type Access = 'read' | 'supervised' | 'full'

export type HistoryEntry = { state: State; at: number }

export type PR = {
  number: number
  url: string
  state: 'draft' | 'open' | 'merged' | 'closed'
  checks: 'none' | 'pending' | 'passing' | 'failing'
  comments: number
  updatedAt: number
  conflicting: boolean
  title: string
  review: '' | 'approved' | 'changes' | 'commented'
  failedChecks: string[]
  runningChecks: number
  passedChecks: number
  newComments: number
}

// RepoPull is an open pull request of the repository; thread is the thread that tracks its branch, '' for none.
export type RepoPull = {
  number: number
  title: string
  draft: boolean
  branch: string
  url: string
  author: string
  updatedAt: number
  thread: string
}

// RepoPulls is the open pull requests the app knows, and why the latest read of them failed, '' when it did not.
export type RepoPulls = { pulls: RepoPull[]; error: string }

export type BranchStatus = {
  base: string
  unpushed: number
  uncommitted: number
  ahead: number
  behind: number
  notInMain: number
}

// FileEntry is a file or folder of the Files view's tree: its path, relative to the folder, and whether git ignores it
// (desktop/chat.FileEntry).
export type FileEntry = { path: string; dir: boolean; ignored: boolean }

// FileListing is entries of a folder, sorted by path; truncated says there are more (desktop/chat.FileListing).
export type FileListing = { entries: FileEntry[]; truncated: boolean }

// FileContent is a file as the Files view shows it (desktop/chat.FileContent). A text file has its first 1 MiB at most
// in text, and truncated says it was cut; size is its own size in bytes. An image has its mime type and its bytes as
// base64 in data. A binary file has neither.
export type FileContent = { kind: 'text' | 'image' | 'binary'; text: string; size: number; truncated: boolean; mime: string; data: string }

export type Session = {
  id: string
  project: string
  n: number
  title: string
  state: State
  detail: string
  lastEventAt: number
  createdAt: number // the thread's first start
  wokeAt: number // its latest start: the first one, or its latest wake
  issue: number
  model: string
  effort: string
  history: HistoryEntry[]
  branch: string
  worktree: string
  pr: PR | null
  git: BranchStatus | null
  prAttention: '' | 'checks' | 'comments'
  cleanup: '' | 'pending' | 'blocked' | 'ask'
  cleanupReason: string
  browser: boolean
  evidence: number
  access: Access
  action: string
  forkOf: string // the thread this one was forked from, '' when none
  forkAt: string // the uuid of the reply the fork's chat was cut at, '' when none
  parent: string // the thread that delegated to this one, '' for a thread the owner started
  side: boolean // a side chat of the thread forkOf, which shows only in that thread's right panel
  tokens: number
  cost: number
  memory: number
  subagents: number // the subagents its Claude runs in the background, which a settle stops
}

// What a delete of a thread would take with it, as DeletePlan says.
export type DeletePlan = {
  worktrees: number // the worktrees orchestrator made for the thread and its descendants that no other thread and no shell works in
  children: string[] // the titles of the asleep descendants that go with the thread, nearest first
}

// One port a server the thread started listens on.
// started is when the server's process started, in ms since the epoch.
export type Server = {
  port: number
  pid: number
  command: string
  started: number
}

export type Memory = {
  app: number
  browsers: number
  sessions: number
  total: number
  machine: number
}

export type Project = {
  key: string
  name: string
  dir: string
  repo: string
  needsYou: number
  working: number
  sessions: number
  needsSetup: boolean
  setupDismissed: boolean
}

// ModelChoice is a model and an effort; '' where nothing sets one.
export type ModelChoice = { model: string; effort: string }

// Place is where a new thread runs: its own worktree or the project folder.
export type Place = 'worktree' | 'project'

// ThreadStart is what the owner chose for a new thread; empty fields leave the choice to the defaults.
export type ThreadStart = { prompt: string; model: string; effort: string }

export type Note = {
  id: string
  text: string
  createdAt: number
  updatedAt: number
  pinned: boolean
  archived: boolean
  images: string[]
}

export type IssueType = 'bug' | 'feature' | 'refactor' | 'chore' | ''

export type Issue = {
  number: number
  title: string
  type: IssueType
  section: string
  actions: string[]
  // access is the access the default action starts its thread with.
  access: Access
  moves: string[]
  url: string
  sessionId: string
  author: string
  assignees: string[]
  labels: string[]
  ignored: string[]
  createdAt: number
  updatedAt: number
}

export type IssueComment = {
  author: string
  createdAt: number
  bodyHTML: string
}

export type IssueDetail = {
  number: number
  bodyHTML: string
  comments: IssueComment[]
}

// Shell is one shell of the current project, with the folder it started in. command is the process in
// front in it, such as zsh, vim or node; busy says it is not the login shell.
export type Shell = { id: string; folder: string; command: string; busy: boolean }

export type Snapshot = {
  project: Project
  projects: Project[]
  sessions: Session[]
  notes: Note[]
  version: string
  shells: Shell[]
  machine: string
}

// installed: the new bundle is in place, and only the restart into it failed.
export type UpdateStatus = 'idle' | 'checking' | 'available' | 'downloading' | 'downloaded' | 'failed' | 'installed'

// version is the newer release, '' when none is known. percent is how far its download got, -1 while the size is
// unknown. failedStep says which step a failed state failed at, '' in any other state. error is why the last action
// failed.
export type UpdateState = { status: UpdateStatus; version: string; percent: number; failedStep: '' | 'download' | 'install'; error: string }

export type ThemeSetting = 'system' | 'light' | 'dark'

export type DarkTheme = 'black' | 'violet'

export type LightTheme = 'light'

export type ThemeName = DarkTheme | LightTheme

export type CleanupMode = 'auto' | 'manual'

export type CleanupSettings = { merge: CleanupMode; close: CleanupMode }

export type CleanupEvent = keyof CleanupSettings

export type DigestSchedule = 'weekly' | 'off'

export type PromptSend = 'auto' | 'manual'

export type Settings = {
  theme: ThemeSetting
  darkTheme: DarkTheme
  lightTheme: LightTheme
  textScale: number
  keepAwake: boolean
  cleanup: CleanupSettings
  promptSend: PromptSend
  browserEnabled: boolean
  digestSchedule: DigestSchedule
  // threadPlace is where a new thread with no issue starts and threadAccess the access it starts with; '' is unset,
  // a worktree and Supervised.
  threadPlace: Place | ''
  threadAccess: Access | ''
  dataDir: string
  dataDirFixed: boolean
  // editorApp is the macOS app that opens a thread's folder.
  editorApp: string
  // projectsDir is the folder a new project is made in; '' is unset, ~/code.
  projectsDir: string
}

// NewProjectOptions is what the add-project view needs before it shows.
export type NewProjectOptions = {
  root: string
  nameMax: number
  // startFolder is where the folder browser starts, with a trailing slash.
  startFolder: string
}

export type FolderEntry = { name: string; path: string }

// FolderListing is the folders in a folder; dir and parent end in a slash and parent is '' for the root.
export type FolderListing = { dir: string; parent: string; exists: boolean; entries: FolderEntry[] }

export type NewProject = { snapshot: Snapshot; dir: string; commitError: string }

export type DataDirChoice = {
  dir: string
  empty: boolean
}

export type LedgerRow = {
  project: string
  prompts: number
  sessions: number
  issueSessions: number
  workMs: number
  prsOpened: number
  prsMerged: number
  prsClosed: number
  issuesFiled: number
  issuesClosed: number
  sessionMs: number
  avgSessionMs: number
  avgLeadMs: number
  tokens: number
  cost: number
}

export type UsageRow = {
  model: string
  effort: string
  sessions: number
  tokens: number
  input: number
  output: number
  cacheRead: number
  cacheWrite: number
  thinking: number
  cost: number
}

export type WorkRow = {
  project: string
  issue: number
  session: string
  title: string
  sessions: number
  tokens: number
  cost: number
  lastAt: number
}

export type DayFigures = {
  day: string
  prompts: number
  workMs: number
  cost: number
}

export type Ledger = {
  days: number
  since: string
  totals: LedgerRow
  projects: LedgerRow[]
  previous: LedgerRow
  perDay: DayFigures[]
  usage: { totals: UsageRow; tasks: UsageRow; byModel: UsageRow[]; pricesAsOf: string }
  work: WorkRow[]
}

// LimitWindow is one plan limit: name is five_hour, seven_day, model:<name> or spend; left is the percent left, 0 to
// 100; resetsAt is unix ms, 0 when Claude does not say.
export type LimitWindow = {
  name: string
  label: string
  left: number
  resetsAt: number
}

// Limits are the plan limits as last read (desktop/limits.Limits). at is when a value last came, unix ms, 0 before any;
// unsupported says that Claude Code does not report its plan usage, so the windows are what its turns told of; error is
// why the latest read failed, '' when it did not.
export type Limits = {
  windows: LimitWindow[]
  at: number
  unsupported: boolean
  error: string
}

export type Cleanup = {
  at: number
  sessionTitle: string
  issue: number
  pr: number
  merged: boolean
  status: 'done' | 'blocked'
  removed: string[]
  reason: string
}

export type BrowserState = {
  id: string
  open: boolean
  url: string
  title: string
  loading: boolean
  error: string
  loadedAt: number
  pages: number
  console: string[]
}

export type Evidence = {
  id: string
  kind: 'image' | 'text'
  url: string
  text: string
  caption: string
  source: 'agent' | 'user'
  at: number
}

export type DigestItem = {
  id: string
  title: string
  why: string
  url: string
  source: string
  at: number
  noteId: string
}

export type Digest = {
  running: boolean
  startedAt: number
  tools: number
  lastRunAt: number
  nextRunAt: number
  error: string
  stoppedAt: number
  project: string
  items: DigestItem[]
}

export type Change = {
  scope: string
  text: string
  issue: number
  url: string
}

export type ReleaseSection = { title: string; changes: Change[] }

export type Release = {
  version: string
  date: string
  sections: ReleaseSection[]
}

// ChatKind is what a chat event is. A delta, a thinking event (a thinking block's start, with no text, or a piece of its
// text) and a thought with no text are never logged and have seq 0; a thought ends a thinking block, with its text. A
// task_start and task_end are a task Claude runs in the background, beside its turns, starting and ending, a subagent
// or a shell among them: parent is the call that started it, and a task_start's tool is its kind, local_bash for a
// shell. A tasks event is Claude's whole list of them, input a list of {id, kind, agent}. A cancel is Claude
// withdrawing the approval id: it no longer waits on an answer.
export type ChatKind =
  | 'init'
  | 'user'
  | 'delta'
  | 'thinking'
  | 'thought'
  | 'message'
  | 'tool_use'
  | 'tool_result'
  | 'approval'
  | 'answer'
  | 'turn_end'
  | 'draft'
  | 'queued'
  | 'unsent'
  | 'interrupt'
  | 'commands'
  | 'compact'
  | 'mode'
  | 'start'
  | 'task_start'
  | 'task_end'
  | 'rewind'
  | 'exit'
  | 'cancel'
  | 'tasks'

// ChatEvent is one event of a thread's chat; a field the event does not use is left out.
export type ChatEvent = {
  seq: number
  at: number
  kind: ChatKind
  id?: string
  uuid?: string
  parent?: string
  text?: string
  tool?: string
  input?: unknown
  error?: boolean
  cost?: number
  files?: string[]
  // stop is set on an exit the host made itself, which is no error: a settle, or a restart whose text is the access
  // the thread starts again with; and on a task_end, for a task that was stopped.
  stop?: 'settle' | 'restart' | 'stopped'
  // agent is the id of the Agent call whose subagent sent the event, missing for the thread's own Claude.
  agent?: string
  // from is the thread that sent a user or queued message, or that asked for an interrupt; left out for the owner or
  // the app.
  from?: string
  // body is the bare words of a message that text frames for Claude, one from a thread or one that carries side chats,
  // and result marks a child's result.
  body?: string
  result?: boolean
  // cwd, origin and model are an init's: the folder Claude works in, the folder it started in and the model it runs.
  cwd?: string
  origin?: string
  model?: string
  // cut is set on a tool call or result whose text the app cut at 64 KB for the window; the text ends with "(cut at 64 KB)".
  cut?: boolean
}

// ChatImage is an image attached to a message: its bytes in base64 and its type.
export type ChatImage = { base64: string; mime: string }

// FilesRewind is what Edit from here offers for a thread's files. offered is false for a thread in the project folder,
// whose files a rewind never touches. reason says why they cannot be rewound, '' when they can; files, by absolute
// path, insertions and deletions are what a rewind of them would change.
export type FilesRewind = { offered: boolean; reason: string; files: string[]; insertions: number; deletions: number }

// ChatSent is a message the thread took: the id it is logged under, and what the user should know when only its text
// went out.
export type ChatSent = { id: string; notice?: string }

// Pending is the approval a thread waits on; id is the thread's and request the approval's. input is the tool's, for a
// card that answers the approval away from the thread's chat; null when the log holds none, or when its JSON is over
// 64 KiB, which inputTooLarge says.
export type Pending = {
  id: string
  request: string
  tool: string
  summary: string
  kind: 'tool' | 'question' | 'plan'
  input: unknown
  inputTooLarge: boolean
}

export type ChatUpdate = { id: string; events: ChatEvent[] }

export type ProjectList<T> = { project: string; items: T[] }

export type Warning = {
  source: 'tmux' | 'github' | 'pull requests' | 'worktrees' | 'config' | 'agent'
  message: string
}

export type EventMap = {
  // A thread's own events; its id holds a slash, which keeps 'chat:pending' out of the pattern.
  [k: `chat:${string}/${string}`]: ChatUpdate
  'chat:pending': { id: string; pending: Pending | null }
  sessions: ProjectList<Session>
  shells: ProjectList<Shell>
  projects: Project[]
  notes: ProjectList<Note>
  issues: ProjectList<Issue>
  cleanups: ProjectList<Cleanup>
  warnings: Warning
  awake: boolean
  memory: Memory
  network: { online: boolean }
  evidence: { id: string; items: Evidence[] }
  digest: Digest
  'browser:state': BrowserState
  'term:exit': { id: string }
  attention: {
    id: string
    state: 'waiting' | 'replied' | 'opened' | 'pr' | 'evidence'
  }
  update: UpdateState
  limits: Limits
}
