export type State = 'waiting' | 'working' | 'idle' | 'ended'

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

export type BranchStatus = {
  base: string
  unpushed: number
  uncommitted: number
  ahead: number
  behind: number
  notInMain: number
}

export type Session = {
  id: string
  project: string
  n: number
  title: string
  state: State
  detail: string
  lastEventAt: number
  createdAt: number
  beganAt: number
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
  chat: boolean
  hosted: boolean
  action: string
  forkOf: string // the thread this one was forked from, '' when none
  tokens: number
  cost: number
  resumable: boolean
  memory: number
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

export type Snapshot = {
  project: Project
  projects: Project[]
  sessions: Session[]
  notes: Note[]
  version: string
  update: string
  shells: string[]
  machine: string
}

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
  dataDir: string
  dataDirFixed: boolean
}

export type DataDirChoice = {
  dir: string
  empty: boolean
}

export type WaitKind = 'permission' | 'question' | 'idle'

export type Wait = {
  sessionTitle: string
  issue: number
  model: string
  kind: WaitKind
  label: string
  startedAt: number
  waitedMs: number
}

export type Stats = {
  days: number
  total: number
  totalWaitMs: number
  medianWaitMs: number
  byCause: { kind: WaitKind; label: string; count: number; totalWaitMs: number }[]
  byDay: { day: string; count: number }[]
  recent: Wait[]
}

export type LedgerRow = {
  project: string
  prompts: number
  chats: number
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

export type Ledger = {
  days: number
  since: string
  totals: LedgerRow
  projects: LedgerRow[]
  heat: { day: string; count: number }[]
  usage: { totals: UsageRow; tasks: UsageRow; byModel: UsageRow[]; pricesAsOf: string }
  work: WorkRow[]
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
  lastRunAt: number
  nextRunAt: number
  error: string
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

// ChatKind is what a chat event is; a delta is never logged and has seq 0.
export type ChatKind =
  | 'init'
  | 'user'
  | 'delta'
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
  | 'exit'

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
}

// ChatImage is an image attached to a message: its bytes in base64 and its type.
export type ChatImage = { base64: string; mime: string }

// ChatSent is a message the thread took: the id it is logged under, and what the user should know when only its text
// went out.
export type ChatSent = { id: string; notice?: string }

// Pending is the approval a thread waits on; id is the thread's and request the approval's.
export type Pending = {
  id: string
  request: string
  tool: string
  summary: string
  kind: 'tool' | 'question' | 'plan'
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
  projects: Project[]
  notes: ProjectList<Note>
  issues: ProjectList<Issue>
  cleanups: ProjectList<Cleanup>
  warnings: Warning
  awake: boolean
  memory: Memory
  stats: undefined
  network: { online: boolean }
  'ui:command': { name: string; args: string[] }
  evidence: { id: string; items: Evidence[] }
  digest: Digest
  'browser:state': BrowserState
  'term:data': { id: string; data: string }
  'term:exit': { id: string }
  attention: {
    id: string
    state: 'waiting' | 'replied' | 'opened' | 'pr' | 'evidence'
  }
  update: { version: string }
}
