import { actionAccess, buildIssueDetail, buildIssues, movesFrom, moveIssue, sortBySection } from './mockIssues'
import { clickAt, newPage, normalizeUrl, pageRects, pageTitle, renderPage, typeText } from './mockBrowser'
import type { PageModel } from './mockBrowser'
import { mockChat } from './mockChat'
import { branchFromTitle, freeBranch } from './branch'
import type { Access, BrowserState, Cleanup, CleanupEvent, CleanupMode, DarkTheme, DeletePlan, Digest, DigestItem, DigestSchedule, EventMap, Evidence, HistoryEntry, Issue, LightTheme, Ledger, LedgerRow, Memory, Note, BranchStatus, Place, PR, Project, Release, Server, Session, PromptSend, Settings, Snapshot, State, Stats, ThemeSetting, UsageRow, Wait, WaitKind, WorkRow } from './types'

type Handler = (payload: never) => void

type ProjectData = { name: string; dir: string; repo: string; notes: Note[]; issues: Issue[]; waits: Wait[]; cleanups: Cleanup[]; needsSetup?: boolean; setupDismissed?: boolean }

const minute = 60_000

// A project that is not set up has no queue sections, so every issue sits in the nameless group.
const unsorted = (issues: Issue[]) => issues.map((issue) => ({ ...issue, section: '', actions: ['Start'], access: actionAccess('Start'), moves: [] }))
const noteTexts = [
  'Retry banner when the websocket drops\nShow a thin bar under the header and retry with backoff. Maybe reuse the toast style so it does not feel like a third kind of notice.\nThe backoff should reset on the first successful frame, not on connect, because the proxy accepts and then drops.',
  'Ask Alex about the checkout button copy. They said something about "Buy now" vs "Pay" last week and I forgot which way it went.',
  'Try a keyboard shortcut sheet in the shop, like the one here. `?` opens it, Esc closes. See https://github.com/acme/storefront/issues/436 for the dark mode thread, same overlay could host it.',
  'Shipping: should the postcode match be case-insensitive?\nCheck how the address lookup returns UK postcodes. I think it keeps the space but upper-cases the letters, which is fine, but older orders might differ.',
  'Idea: weekly sales as a sparkline in the stats table',
  'Rename Catalogue to Shop everywhere in the copy. Check the emails too, the order template still says Catalogue.',
  'Write the migration note for the price table: old flat map goes away, priceFor(currency, region). Call out that the payment provider only needs currency + amount.',
  'Check the image upload size limit (10MB on the CDN)',
]

const shot = (hue: number, label: string) =>
  `data:image/svg+xml;utf8,${encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="200" viewBox="0 0 320 200"><rect width="320" height="200" fill="hsl(${hue} 30% 16%)"/><rect x="16" y="16" width="288" height="22" fill="hsl(${hue} 40% 26%)"/><rect x="16" y="52" width="180" height="12" fill="hsl(${hue} 50% 40%)"/><rect x="16" y="76" width="260" height="8" fill="hsl(${hue} 20% 30%)"/><rect x="16" y="94" width="240" height="8" fill="hsl(${hue} 20% 30%)"/><rect x="16" y="112" width="200" height="8" fill="hsl(${hue} 20% 30%)"/><text x="16" y="178" fill="hsl(${hue} 60% 75%)" font-family="monospace" font-size="14">${label}</text></svg>`,
  )}`

function seedNotes(texts: string[]): Note[] {
  const now = Date.now()
  return texts.map((text, i) => ({
    id: `note-${Math.random().toString(36).slice(2, 8)}`,
    text,
    createdAt: now - (i + 1) * 37 * minute,
    updatedAt: now - (i + 1) * 37 * minute,
    pinned: i === 2,
    archived: i >= texts.length - 2,
    images: i === 0 ? [shot(210, 'before: banner overlaps'), shot(150, 'after: thin bar')] : i === 3 ? [shot(30, 'postcode lookup')] : [],
  }))
}

const causes: { kind: WaitKind; label: string; weight: number; waitMin: number }[] = [
  { kind: 'permission', label: 'Bash: npm run test', weight: 18, waitMin: 2 },
  { kind: 'permission', label: 'Bash: npx tsc --noEmit', weight: 14, waitMin: 1 },
  { kind: 'permission', label: 'Edit', weight: 12, waitMin: 1 },
  { kind: 'permission', label: 'Bash: gh pr create', weight: 6, waitMin: 3 },
  { kind: 'permission', label: 'Read outside project', weight: 5, waitMin: 2 },
  { kind: 'question', label: 'Question', weight: 10, waitMin: 7 },
  { kind: 'idle', label: 'Reply landed', weight: 16, waitMin: 11 },
]

function seedWaits(titles: [string, number][]): Wait[] {
  const now = Date.now()
  let seedValue = 7
  const random = () => {
    seedValue = (seedValue * 16807) % 2147483647
    return seedValue / 2147483647
  }
  const total = causes.reduce((sum, c) => sum + c.weight, 0)
  const waits: Wait[] = []
  for (let d = 14; d >= 0; d--) {
    const weekend = new Date(now - d * 86_400_000).getDay() % 6 === 0
    const count = Math.round((weekend ? 2 : 7) + random() * 6)
    for (let i = 0; i < count; i++) {
      let roll = random() * total
      const cause = causes.find((c) => (roll -= c.weight) < 0) ?? causes[0]
      const [sessionTitle, issue] = titles[Math.floor(random() * titles.length)]
      waits.push({
        sessionTitle,
        issue,
        model: 'sonnet',
        kind: cause.kind,
        label: cause.label,
        startedAt: now - d * 86_400_000 - Math.floor(random() * 9 * 3_600_000),
        waitedMs: Math.round((cause.waitMin * (0.4 + random() * 1.6)) * minute),
      })
    }
  }
  return waits.sort((a, b) => b.startedAt - a.startedAt)
}

function pr(number: number, state: PR['state'], checks: PR['checks'], extra: Partial<PR> = {}): PR {
  return {
    number,
    url: `https://github.com/acme/storefront/pull/${number}`,
    state,
    checks,
    comments: 0,
    updatedAt: Date.now() - 3 * minute,
    conflicting: false,
    title: `Pull request ${number}`,
    review: '',
    failedChecks: [],
    runningChecks: checks === 'pending' ? 2 : 0,
    passedChecks: checks === 'passing' || checks === 'pending' ? 4 : 0,
    newComments: 0,
    ...extra,
  }
}

function git(extra: Partial<BranchStatus> = {}): BranchStatus {
  return { base: 'main', unpushed: 0, uncommitted: 0, ahead: 1, behind: 0, notInMain: 0, ...extra }
}

const details: Record<State, string[]> = {
  working: [
    'Edit route-list.tsx',
    'Bash npx tsc --noEmit',
    'Read Layout.tsx',
    'Grep productContext',
    'Edit Index.tsx',
    'Write ProductGallery.tsx',
  ],
  waiting: ['Claude needs your permission to use Bash: npx tsc --noEmit', 'Should I keep the old cart hook or replace it?'],
  idle: ['Done: 4 files changed', 'Done: tests pass'],
  settled: ['Thread asleep'],
}

const order: Record<State, number> = { waiting: 0, idle: 1, working: 2, settled: 3 }

// Claude's permission mode for each access, and the access each mode it reports gives, as the agent package has them.
const modeOfAccess: Record<Access, string> = { read: 'plan', supervised: 'default', full: 'bypassPermissions' }
const accessOfMode: Record<string, Access> = { plan: 'read', default: 'supervised', acceptEdits: 'supervised', bypassPermissions: 'full' }

const pick = <T,>(items: T[]): T => items[Math.floor(Math.random() * items.length)]

function toBase64(text: string): string {
  const bytes = new TextEncoder().encode(text)
  let binary = ''
  for (let i = 0; i < bytes.length; i += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
  }
  return btoa(binary)
}

const PATCH_NOTES: Release[] = [
  {
    version: '0.14.0',
    date: '2026-10-07',
    sections: [
      { title: 'Features', changes: [{ scope: 'desktop', text: 'let agents close their browser window', issue: 218, url: 'https://github.com/nednella/agentos/issues/218' }] },
      {
        title: 'Bug Fixes',
        changes: [
          { scope: 'desktop', text: 'forget browser whose connection dropped', issue: 218, url: 'https://github.com/nednella/agentos/issues/218' },
          { scope: 'desktop', text: "start browser without last run's pages", issue: 218, url: 'https://github.com/nednella/agentos/issues/218' },
          { scope: 'desktop', text: 'stop browser once its last window closes', issue: 218, url: 'https://github.com/nednella/agentos/issues/218' },
        ],
      },
    ],
  },
  {
    version: '0.13.0',
    date: '2026-10-07',
    sections: [
      {
        title: 'Features',
        changes: [
          { scope: 'cli', text: 'add --prompt to agentos new', issue: 0, url: '' },
          { scope: 'desktop', text: 'tell claude sessions how to start a session', issue: 0, url: '' },
        ],
      },
    ],
  },
  {
    version: '0.12.0',
    date: '2026-10-07',
    sections: [
      { title: 'Features', changes: [{ scope: 'build', text: 'draw terminal prompt app icon', issue: 0, url: '' }] },
      { title: 'Bug Fixes', changes: [{ scope: 'ui', text: 'pad top of queue list', issue: 0, url: '' }] },
      { title: 'Performance Improvements', changes: [{ scope: 'desktop', text: 'start chrome-devtools-mcp straight from the npx cache', issue: 0, url: '' }] },
    ],
  },
]

export function createMock(params: URLSearchParams) {
  const flags = {
    empty: params.has('empty'),
    overlay: params.get('overlay') ?? undefined,
    tab: params.get('tab') ?? undefined,
    cmd: params.get('cmd') ?? undefined,
    view: params.get('view') ?? undefined,
    toast: params.has('toast'),
    done: params.has('done'),
    warn: params.has('warn'),
    offline: params.has('offline'),
    update: params.has('update'),
    patchNotes: params.has('patchnotes'),
    fullDataDir: params.has('fulldata'),
    noModel: params.has('nomodel'),
    longChat: params.has('longchat'),
    chatCases: params.has('chatcases'),
    staleAnswer: params.has('stale'),
    // lineage gives #454 a delegated child and a fork, for its context card.
    lineage: params.has('lineage'),
    // childask=question or childask=plan has the child ask that; any other value a tool approval.
    childAsk: params.get('childask'),
  }
  const handlers = new Map<string, Set<Handler>>()
  const sessions: Session[] = []
  const data: ProjectData[] = [
    { name: 'storefront', dir: '~/code/acme/storefront', repo: 'acme/storefront', notes: seedNotes(noteTexts), issues: buildIssues('acme/storefront'), waits: seedWaits([['#454 product page split', 454], ['#444 checkout route', 444], ['#326 cart rounding', 326], ['#389 webhook retries', 389], ['checkout tests scratch', 0]]), cleanups: [{ at: Date.now() - 2 * 3_600_000, sessionTitle: '#412 locale sweep', issue: 412, pr: 441, merged: true, status: 'blocked', removed: [], reason: 'the worktree of issue-412 has uncommitted changes' }, { at: Date.now() - 5 * 3_600_000, sessionTitle: '#371 old banner', issue: 371, pr: 440, merged: true, status: 'done', removed: ['worktree ~/code/acme/storefront/trees/issue-371', 'thread put to sleep', 'temp folder'], reason: '' }, { at: Date.now() - 26 * 3_600_000, sessionTitle: '#366 retry copy', issue: 366, pr: 431, merged: true, status: 'done', removed: ['worktree ~/code/acme/storefront/trees/issue-366', 'thread put to sleep', 'temp folder'], reason: '' }] },
    { name: 'agentos', dir: '~/code/agentos', repo: 'nednella/agentos', notes: seedNotes(noteTexts.slice(0, 3)), issues: unsorted(buildIssues('nednella/agentos')), waits: seedWaits([['#12 session memory', 12]]).slice(0, 12), cleanups: [], needsSetup: true },
    { name: 'scratch', dir: '~/scratch', repo: '', notes: [], issues: [], waits: [], cleanups: [] },
  ]
  let current = data[0]
  let nextN = 1
  let nextIssue = 460

  function emit<K extends keyof EventMap>(event: K, payload: EventMap[K]) {
    handlers.get(event)?.forEach((handler) => handler(payload as never))
  }

  function on<K extends keyof EventMap>(event: K, handler: (payload: EventMap[K]) => void) {
    const set = handlers.get(event) ?? new Set<Handler>()
    set.add(handler as Handler)
    handlers.set(event, set)
    return () => set.delete(handler as Handler)
  }

  const tenthGB = 2 ** 30 / 10 // the app rounds figures up to this

  function view(s: Session): Session {
    return {
      id: s.id,
      project: s.project,
      n: s.n,
      title: s.title,
      state: s.state,
      detail: s.detail,
      lastEventAt: s.lastEventAt,
      createdAt: s.createdAt,
      wokeAt: s.wokeAt,
      issue: s.issue,
      model: s.model,
      effort: s.effort,
      history: s.history.map((h) => ({ ...h })),
      branch: s.branch,
      worktree: s.worktree,
      pr: s.pr ? { ...s.pr, failedChecks: [...s.pr.failedChecks] } : null,
      git: s.git ? { ...s.git } : null,
      prAttention: s.prAttention,
      cleanup: s.cleanup,
      cleanupReason: s.cleanupReason,
      browser: s.browser,
      evidence: s.evidence,
      access: s.access,
      hosted: s.hosted,
      action: s.action,
      forkOf: s.forkOf,
      parent: s.parent,
      tokens: s.tokens,
      cost: s.cost,
      memory: s.state === 'settled' ? 0 : s.memory,
      subagents: s.state === 'settled' ? 0 : s.subagents,
    }
  }

  function memory(): Memory {
    const app = 2 * tenthGB
    const browsers = 11 * tenthGB
    const live = sessions.filter((s) => s.state !== 'settled').reduce((sum, s) => sum + s.memory, 0)
    return { app, browsers, sessions: live, total: app + browsers + live, machine: 16 * 2 ** 30 }
  }

  function sessionsOf(project: string): Session[] {
    return sessions
      .filter((s) => s.project === project)
      .map(view)
      .sort((a, b) => order[a.state] - order[b.state] || b.lastEventAt - a.lastEventAt)
  }

  function projectView(p: ProjectData): Project {
    const own = sessions.filter((s) => s.project === p.name)
    return {
      key: p.name,
      name: p.name,
      dir: p.dir,
      repo: p.repo,
      needsYou: own.filter((s) => s.state === 'waiting').length,
      working: own.filter((s) => s.state === 'working').length,
      sessions: own.length,
      needsSetup: !!p.needsSetup,
      setupDismissed: !!p.setupDismissed,
    }
  }

  function currentNotes(): Note[] {
    return [...current.notes].sort(
      (a, b) => Number(a.archived) - Number(b.archived) || Number(b.pinned) - Number(a.pinned) || b.createdAt - a.createdAt,
    )
  }

  const lastMoves = new Map<string, Issue>()

  function currentIssues(): Issue[] {
    return current.issues.map((issue) => ({
      ...issue,
      sessionId: sessions.find((s) => s.project === current.name && !s.forkOf && s.issue === issue.number)?.id ?? '',
    }))
  }

  type Shell = { id: string; project: string; dir: string; buffer: string; line: string; opened: boolean }
  const shells: Shell[] = []
  const shellName = (project: string, n: number) => (n === 1 ? `shell-${project}` : `shell-${project}-${n}`)

  const prompt = (project: string) => `\x1b[38;2;167;139;250m${project}\x1b[0m \x1b[2m❯\x1b[0m `

  function shellWrite(shell: Shell, text: string) {
    shell.buffer += text
    if (shell.opened) emit('term:data', { id: shell.id, data: toBase64(text) })
  }

  function shellRun(shell: Shell, line: string) {
    const [name, ...args] = line.trim().split(/\s+/)
    if (!name) return
    if (name === 'agentos') {
      const [command = '', ...rest] = args
      emit('ui:command', { name: command, args: rest })
      shellWrite(shell, `\r\n\x1b[2mok: agentos ${args.join(' ')}\x1b[0m`)
      return
    }
    if (name === 'ls') shellWrite(shell, '\r\napp-frontend  api  e2e  locales  CLAUDE.md  package.json')
    else if (name === 'pwd') shellWrite(shell, `\r\n${shell.dir}`)
    else if (name === 'clear') shellWrite(shell, '\x1bc')
    else shellWrite(shell, `\r\nzsh: command not found: ${name}`)
  }

  const shellsOf = (project: string) => shells.filter((x) => x.project === project)
  const findShell = (id: string) => shells.find((x) => x.id === id)

  function startShell(sessionId: string): Shell {
    const dir = sessions.find((x) => x.id === sessionId)?.worktree || current.dir
    const taken = shellsOf(current.name).map((x) => x.id)
    let n = 1
    while (taken.includes(shellName(current.name, n))) n++
    const shell: Shell = { id: shellName(current.name, n), project: current.name, dir, buffer: `\x1b[2m${dir}\x1b[0m\r\n${prompt(current.name)}`, line: '', opened: false }
    shells.push(shell)
    return shell
  }

  function snapshot(): Snapshot {
    return {
      project: projectView(current),
      projects: data.map(projectView),
      sessions: data.flatMap((p) => sessionsOf(p.name)),
      notes: currentNotes(),
      version: 'mock',
      update: flags.update ? '1.4.0' : '',
      shells: shellsOf(current.name).map((x) => x.id),
      machine: 'Studio MacBook Pro',
    }
  }

  function publish() {
    data.forEach((p) => emit('sessions', { project: p.name, items: sessionsOf(p.name) }))
    emit('projects', data.map(projectView))
  }

  function find(id: string): Session {
    const found = sessions.find((s) => s.id === id)
    if (!found) throw 'No such session'
    return found
  }

  function modelFor(owner: ProjectData, issue: number) {
    const found = owner.issues.find((i) => i.number === issue)
    const labelled = (key: string) => found?.labels.find((l) => l.startsWith(`${key}:`))?.slice(key.length + 1)
    return {
      model: labelled('model') ?? (found?.section === 'Needs plan' ? 'opus' : 'sonnet'),
      effort: labelled('effort') ?? 'medium',
    }
  }

  function usageFor(createdAt: number) {
    const tokens = 180_000 + (Math.floor(createdAt / 1000) % 7) * 410_000
    return { tokens, cost: tokens * 0.0000021 }
  }

  function create(
    title: string,
    issue: number,
    initial: State,
    history: HistoryEntry[] = [],
    createdAt = Date.now(),
    owner = current,
    access: Access = 'supervised',
  ): Session {
    const s: Session = {
      id: `${owner.name}/${Math.random().toString(36).slice(2, 10)}`,
      n: nextN++,
      title,
      state: initial,
      detail: pick(details[initial]),
      lastEventAt: history.at(-1)?.at ?? 0,
      createdAt,
      wokeAt: createdAt,
      issue,
      ...modelFor(owner, issue),
      history,
      branch: issue ? `issue-${issue}` : '',
      worktree: issue ? `${owner.dir}/trees/issue-${issue}` : '',
      pr: null,
      git: null,
      prAttention: '',
      cleanup: '',
      cleanupReason: '',
      browser: false,
      evidence: 0,
      access,
      hosted: true,
      action: issue ? 'Work' : '',
      forkOf: '',
      parent: '',
      ...usageFor(createdAt),
      memory: (4 + (nextN % 5) * 3) * tenthGB,
      subagents: 0,
      project: owner.name,
    }
    sessions.push(s)
    return s
  }

  // Two seeded threads have a chat, one waiting on a tool and one on a question; the others start with an empty one,
  // and one ran in a terminal before agentos showed chats, so it has none. With chatcases, one chat log cannot be read
  // and one more thread waits on a plan. With stale, the tool's approval ends on the host's side and the app is not told.
  const deleted = new Set<string>()
  const chats = mockChat(emit, (id) => {
    if (deleted.has(id)) throw `Thread ${id} was deleted`
    const s = find(id)
    if (!s.hosted) throw 'This thread ran in a terminal before agentos showed chats'
    return {
      settled: s.state === 'settled',
      worktree: s.worktree,
      filesReason: () => filesReason(s),
      setState: (state) => setState(s, state),
      wake: () => wake(s),
      followMode: (mode) => followMode(s, mode),
      setSubagents: (n) => {
        s.subagents = n
        publish()
      },
    }
  }, (key) => data.some((p) => p.name === key))

  // followMode records the access Claude's permission mode gives, as the sessions do.
  function followMode(s: Session, mode: string) {
    const access = accessOfMode[mode]
    if (!access || access === s.access) return
    s.access = access
    publish()
  }

  // wake starts a settled thread again under its id, as the sessions do.
  function wake(s: Session) {
    if (!s.hosted) throw `Thread ${s.id} ran in a terminal and cannot be woken`
    if (s.state !== 'settled') throw `Thread ${s.id} is not asleep`
    s.wokeAt = Date.now()
    setState(s, 'idle')
    chats.start(s.id)
  }

  function seed(title: string, issue: number, state: State, trail: [State, number][], owner = data[0], access: Access = 'supervised') {
    const now = Date.now()
    const history = trail.map(([st, minutesAgo]) => ({ state: st, at: now - minutesAgo * minute }))
    const s = create(title, issue, state, history, history[0] ? history[0].at - minute : now, owner, access)
    s.state = state
    return s
  }

  // The servers each thread started, by its id. They stop when the thread settles or is deleted, as the real ones do.
  const servers = new Map<string, Server[]>()

  if (!flags.empty) {
    seed('#454 product page split', 454, 'working', [['working', 74], ['waiting', 52], ['working', 47], ['idle', 31], ['working', 12]])
    seed('#444 checkout route', 444, 'waiting', [['working', 96], ['idle', 70], ['working', 38], ['waiting', 4]])
    seed('#326 cart rounding', 326, 'idle', [['working', 41], ['waiting', 33], ['working', 30], ['idle', 6]])
    seed('checkout tests scratch', 0, 'working', [['working', 22], ['waiting', 15], ['working', 13]])
    seed('#412 locale sweep', 412, 'idle', [['working', 50], ['idle', 20]])
    seed('#437 coupon strip', 437, 'idle', [['working', 80], ['idle', 25]])
    seed('#430 thumbnail ratio', 430, 'settled', [['working', 60], ['settled', 18]])
    seed('spike: websocket retry', 0, 'settled', [['working', 140], ['idle', 110], ['settled', 95]])
    seed('#389 webhook retries', 389, 'working', [['working', 58], ['idle', 40], ['working', 9]])
    seed('retry banner experiment', 0, 'settled', [['working', 400], ['idle', 380], ['settled', 300]])
    const byIssue = (n: number) => sessions.find((x) => x.issue === n && x.project === 'storefront')!
    Object.assign(sessions.find((x) => x.title === 'spike: websocket retry')!, { hosted: false, detail: 'Ran in a terminal before chats' })
    Object.assign(byIssue(454), { pr: pr(457, 'draft', 'pending', { title: 'Split the product page into sections' }), git: git({ unpushed: 2, uncommitted: 4, ahead: 3 }) })
    servers.set(byIssue(454).id, [
      { port: 5173, pid: 48211, command: 'node /Users/ned/code/acme/storefront/trees/issue-454/node_modules/.bin/vite --port 5173' },
      { port: 8787, pid: 48230, command: 'node server/api.js --watch' },
    ])
    if (flags.lineage) {
      const split = byIssue(454)
      const bundle = seed('Measure the product page bundle', 0, 'working', [['working', 6]], data[0], 'read')
      bundle.parent = split.id
      const fork = seed('#454 product page split (fork)', 454, 'idle', [['working', 20], ['idle', 14]])
      Object.assign(fork, { forkOf: split.id, branch: '', worktree: split.worktree })
    }
    Object.assign(byIssue(389), { pr: pr(459, 'draft', 'passing', { title: 'Retry webhooks with backoff', review: 'approved' }), git: git() })
    Object.assign(byIssue(444), {
      pr: pr(452, 'open', 'failing', { title: 'Move checkout to its own route', review: 'changes', comments: 4, newComments: 2, failedChecks: ['unit-tests', 'lint / eslint'], passedChecks: 3 }),
      prAttention: 'checks',
      git: git({ ahead: 4 }),
    })
    Object.assign(byIssue(326), {
      pr: pr(449, 'open', 'passing', { title: 'Round cart totals half up', conflicting: true, comments: 3, review: 'commented' }),
      git: git({ behind: 9, ahead: 2 }),
    })
    Object.assign(byIssue(412), { pr: pr(441, 'merged', 'passing', { title: 'Sweep unused locale keys' }), git: git({ uncommitted: 3 }), cleanup: 'blocked', cleanupReason: 'the worktree of issue-412 has uncommitted changes' })
    Object.assign(byIssue(437), { pr: pr(445, 'merged', 'passing', { title: 'Show the coupon strip' }), git: git({ notInMain: 1, ahead: 2 }), cleanup: 'pending' })
    Object.assign(byIssue(430), { pr: pr(447, 'closed', 'failing', { title: 'Fix the thumbnail ratio', failedChecks: ['unit-tests'] }), git: git(), cleanup: 'ask' })
    Object.assign(sessions.find((x) => x.title === 'checkout tests scratch')!, { branch: 'scratch-checkout', git: git({ unpushed: 2, ahead: 2 }) })
    Object.assign(sessions.find((x) => x.title === 'retry banner experiment')!, {
      branch: 'experiment/websocket-retry-banner-with-backoff',
      pr: pr(438, 'merged', 'passing', { title: 'Show a retry banner' }),
      git: git({ uncommitted: 2 }),
      cleanup: 'blocked',
      cleanupReason: 'the worktree of experiment/websocket-retry-banner-with-backoff has uncommitted changes',
    })
    seed('#8 drop old flag', 8, 'settled', [['working', 300], ['settled', 240]], data[2])
    // A thread asleep with the two children it delegated to, asleep too: one in a worktree of its own, one in its folder.
    const ratio = byIssue(430)
    Object.assign(seed('Measure the thumbnail sizes', 0, 'settled', [['working', 55], ['settled', 40]]), { parent: ratio.id, branch: 'measure-the-thumbnail-sizes', worktree: '~/code/acme/storefront/trees/measure-the-thumbnail-sizes' })
    Object.assign(seed('Check the image CDN headers', 0, 'settled', [['working', 50], ['settled', 42]], data[0], 'read'), { parent: ratio.id, branch: '', worktree: '' })
    seed('#12 session memory', 12, 'waiting', [['working', 30], ['waiting', 7]], data[1])
    seed('notes sync spike', 0, 'working', [['working', 11]], data[1])
    seed('Why does checkout call the cart API twice?', 0, 'waiting', [['working', 6], ['waiting', 2]], data[0], 'read')
    seed('Where do we set the cache headers?', 0, 'idle', [['working', 30], ['idle', 24]], data[0], 'read')
    // A thread that delegated two tasks: one child in its own project, one in another. With childask, the first
    // waits on a tool approval, and the app hears of it.
    const lead = sessions.find((x) => x.title === 'checkout tests scratch')!
    const finder = seed('Find the flaky checkout test', 0, 'working', [['working', 9]], data[0], 'read')
    finder.parent = lead.id
    if (flags.childAsk !== null) {
      Object.assign(finder, { state: 'waiting', detail: 'Claude needs your permission to use Bash: npm install' })
      if (flags.childAsk === 'question' || flags.childAsk === 'plan') chats.ask(finder.id, flags.childAsk)
      else chats.seed(finder.id)
      setTimeout(() => emit('attention', { id: finder.id, state: 'waiting' }), 1500)
    }
    const notesCheck = seed('Check the notes sync for the same race', 0, 'idle', [['working', 8], ['idle', 3]], data[1], 'read')
    notesCheck.parent = lead.id
    const sender = byIssue(444)
    chats.delegations(
      lead.id,
      [
        { id: finder.id, title: finder.title, task: 'Find which checkout test fails now and then, and why.', access: 'read' },
        {
          id: notesCheck.id,
          title: notesCheck.title,
          task: 'Check whether the notes sync has the race the checkout tests hit.',
          access: 'read',
          project: notesCheck.project,
          result: 'No race in the notes sync ([its test](https://example.com/notes-sync-test)). It writes each note to a temp file and renames it into place, so a reader never sees half a note.\n\n- `sync.ts` holds one lock per note\n- the tests run 200 syncs side by side and all pass\n- the only shared state is the queue, which is read under the same lock\n\n```sh\nnpm test -- notes-sync --repeat 200\n```',
        },
      ],
      { id: sender.id, title: sender.title },
    )
    if (flags.chatCases) seed('Plan the checkout split', 0, 'waiting', [['working', 4], ['waiting', 1]], data[0], 'read')
    byIssue(444).detail = 'Claude needs your permission to use Bash: npm install'
    const brokenChat = byIssue(326)
    const questionChat = sessions.find((x) => x.title === 'Why does checkout call the cart API twice?')!
    const planChat = sessions.find((x) => x.title === 'Plan the checkout split')
    if (flags.chatCases) chats.breakLog(brokenChat.id)
    chats.seed(byIssue(444).id, flags.longChat ? 500 : 0)
    // After the app's first read of what each thread waits on, so it still shows the approval.
    if (flags.staleAnswer) setTimeout(() => chats.forget(byIssue(444).id), 2000)
    questionChat.detail = 'Should I keep the old cart hook or replace it?'
    chats.ask(questionChat.id, 'question')
    if (planChat) {
      planChat.detail = 'Claude needs your permission to use ExitPlanMode'
      chats.ask(planChat.id, 'plan')
    }
  }

  if (flags.warn) {
    setTimeout(() => {
      emit('warnings', { source: 'github', message: 'gh could not name the repo' })
    }, 1500)
    setTimeout(() => emit('warnings', { source: 'github', message: '' }), 9000)
  }

  if (flags.toast) {
    setTimeout(() => {
      const target = sessions[2]
      if (target) emit('attention', { id: target.id, state: 'replied' })
    }, 1500)
    setTimeout(() => {
      const target = sessions.find((s) => s.pr?.state === 'draft')
      if (target) emit('attention', { id: target.id, state: 'opened' })
    }, 2500)
  }

  function owner(s: Session): ProjectData {
    return data.find((p) => p.name === s.project) ?? current
  }

  function closeWait(s: Session, at: number) {
    const open = owner(s).waits.find((w) => w.sessionTitle === s.title && w.waitedMs === 0)
    if (open) open.waitedMs = Math.max(at - open.startedAt, 1000)
  }

  function openWait(s: Session, at: number) {
    const permission = s.detail.includes('permission')
    const kind: WaitKind = s.state === 'idle' ? 'idle' : permission ? 'permission' : 'question'
    const label = { permission: 'Bash: npx tsc --noEmit', question: 'Question', idle: 'Reply landed' }[kind]
    owner(s).waits.unshift({ sessionTitle: s.title, issue: s.issue, model: s.model, kind, label, startedAt: at, waitedMs: 0 })
  }

  function setState(s: Session, state: State) {
    const at = Date.now()
    closeWait(s, at)
    s.state = state
    s.detail = pick(details[state])
    s.lastEventAt = at
    s.history = [...s.history, { state, at }].slice(-200)
    publish()
    if (state === 'waiting' || state === 'idle') {
      openWait(s, at)
      emit('stats', undefined)
      if (s.project === current.name) emit('attention', { id: s.id, state: state === 'idle' ? 'replied' : 'waiting' })
    }
  }

  function noteById(id: string): Note {
    const note = current.notes.find((n) => n.id === id)
    if (!note) throw 'No such note'
    return note
  }

  function touchNotes() {
    emit('notes', { project: current.name, items: currentNotes() })
  }

  const titleFromPrompt = (prompt: string) => prompt.trim().split('\n')[0].trim().slice(0, 60)

  // The thread gets its first message through its chat, as the app gives it to the host.
  function startSessionFor(title: string, issue: number, prompt: string, action = '', model = '', effort = '', access: Access = 'supervised', where: Pick<Session, 'branch' | 'worktree'> | null = null): Session {
    const s = create(title, issue, 'idle', [], Date.now(), current, access)
    if (where) Object.assign(s, where)
    s.action = action
    if (model) s.model = model
    if (effort) s.effort = effort
    chats.start(s.id)
    if (prompt) void chats.backend.ChatSend(s.id, prompt, [])
    publish()
    return view(s)
  }

  function switchTo(next: ProjectData): Snapshot {
    current = next
    return snapshot()
  }

  function statsFor(days: number, project: string): Stats {
    const since = Date.now() - days * 86_400_000
    const waits = data
      .filter((p) => !project || p.name === project)
      .flatMap((p) => p.waits)
      .filter((w) => w.startedAt >= since)
      .sort((a, b) => b.startedAt - a.startedAt)
    const done = waits.filter((w) => w.waitedMs > 0).map((w) => w.waitedMs).sort((a, b) => a - b)
    const groups = new Map<string, { kind: WaitKind; label: string; count: number; totalWaitMs: number }>()
    for (const w of waits) {
      const g = groups.get(w.label) ?? { kind: w.kind, label: w.label, count: 0, totalWaitMs: 0 }
      g.count++
      g.totalWaitMs += w.waitedMs
      groups.set(w.label, g)
    }
    const byDay = Array.from({ length: days }, (_, i) => {
      const day = new Date(Date.now() - (days - 1 - i) * 86_400_000).toISOString().slice(0, 10)
      return { day, count: waits.filter((w) => new Date(w.startedAt).toISOString().slice(0, 10) === day).length }
    })
    return {
      days,
      total: waits.length,
      totalWaitMs: done.reduce((sum, ms) => sum + ms, 0),
      medianWaitMs: done.length ? done[Math.floor(done.length / 2)] : 0,
      byCause: [...groups.values()].sort((a, b) => b.count - a.count),
      byDay,
      recent: waits.slice(0, 50),
    }
  }

  function ledgerFor(days: number, project: string): Ledger {
    const dayKey = (ago: number) => new Date(Date.now() - ago * 86_400_000).toISOString().slice(0, 10)
    const scale = days === 0 ? 1 : 0.05 + 0.75 * (days / 365)
    const row = (project: string, prompts: number): LedgerRow => ({
      project,
      prompts: Math.round(prompts * scale),
      chats: Math.round(prompts * scale * 0.03),
      sessions: Math.round(prompts * scale * 0.08),
      issueSessions: Math.round(prompts * scale * 0.045),
      workMs: Math.round(prompts * scale * 0.13 * 3_600_000),
      prsOpened: Math.round(prompts * scale * 0.05),
      prsMerged: Math.round(prompts * scale * 0.04),
      prsClosed: Math.round(prompts * scale * 0.005),
      issuesFiled: Math.round(prompts * scale * 0.02),
      issuesClosed: Math.round(prompts * scale * 0.035),
      sessionMs: Math.round(prompts * scale * 0.08) * 52 * 60_000,
      avgSessionMs: prompts > 100 ? 52 * 60_000 : 0,
      avgLeadMs: prompts > 100 ? 5 * 3_600_000 : 0,
      tokens: Math.round(prompts * scale * 61_000),
      cost: prompts * scale * 0.083,
    })
    const all = [row('storefront', 3104), row('agentos', 1209), row('scratch', 38)]
    const projects = all.filter((r) => !project || r.project === project)
    const span = days === 0 ? 121 : days
    const shape = Array.from({ length: span }, (_, i) => {
      const ago = span - 1 - i
      const quiet = ago % 7 === 2 || ago % 7 === 3 || ago % 11 === 0
      return { day: dayKey(ago), weight: quiet ? 0 : Math.round((Math.sin(ago / 9) + 1.2) * ((ago * 37) % 23)) }
    })
    const totals: LedgerRow = {
      project: '',
      prompts: projects.reduce((a, r) => a + r.prompts, 0),
      chats: projects.reduce((a, r) => a + r.chats, 0),
      sessions: projects.reduce((a, r) => a + r.sessions, 0),
      issueSessions: projects.reduce((a, r) => a + r.issueSessions, 0),
      workMs: projects.reduce((a, r) => a + r.workMs, 0),
      prsOpened: projects.reduce((a, r) => a + r.prsOpened, 0),
      prsMerged: projects.reduce((a, r) => a + r.prsMerged, 0),
      prsClosed: projects.reduce((a, r) => a + r.prsClosed, 0),
      issuesFiled: projects.reduce((a, r) => a + r.issuesFiled, 0),
      issuesClosed: projects.reduce((a, r) => a + r.issuesClosed, 0),
      sessionMs: projects.reduce((a, r) => a + r.sessionMs, 0),
      avgSessionMs: 52 * 60_000,
      avgLeadMs: 5 * 3_600_000,
      tokens: projects.reduce((a, r) => a + r.tokens, 0),
      cost: projects.reduce((a, r) => a + r.cost, 0),
    }
    const usageRow = (model: string, effort: string, share: number, sessions: number): UsageRow => {
      const tokens = Math.round(totals.tokens * share)
      return {
        model,
        effort,
        sessions,
        tokens,
        input: Math.round(tokens * 0.002),
        output: Math.round(tokens * 0.018),
        cacheRead: Math.round(tokens * 0.9),
        cacheWrite: Math.round(tokens * 0.08),
        thinking: Math.round(tokens * 0.006),
        cost: totals.cost * share,
      }
    }
    const byModel = [
      usageRow('claude-opus-5-5', 'high', 0.62, Math.round(totals.sessions * 0.6)),
      usageRow('claude-opus-5-5', '', 0.21, Math.round(totals.sessions * 0.3)),
      usageRow('claude-sonnet-5-5', 'medium', 0.12, Math.round(totals.sessions * 0.1)),
      usageRow('claude-haiku-5-5', '', 0.05, Math.round(totals.sessions * 0.4)),
    ]
    const hoursAgo = (h: number) => Date.now() - h * 3_600_000
    const titles = ['Retry banner on websocket drop', 'Postcode match ignores case', 'Rename Catalogue to Shop', 'Weekly sales sparkline', 'Dark mode for emails', 'Order export as CSV', 'Faster product search', 'Gift card balance check', 'Stock alerts by email', 'Split the admin bundle']
    const older: WorkRow[] = titles.map((title, i) => ({ project: 'storefront', issue: 400 + i, session: '', title: `#${400 + i} ${title}`, sessions: 1 + (i % 3), tokens: (i + 1) * 1_300_000, cost: (i + 1) * 0.64, lastAt: hoursAgo(60 + i * 20) }))
    const work: WorkRow[] = [
      ...older,
      { project: 'agentos', issue: 320, session: '', title: '#320 Token usage and estimated cost per issue', sessions: 2, tokens: 18_400_000, cost: 9.12, lastAt: hoursAgo(0.2) },
      { project: 'agentos', issue: 316, session: '', title: '#316 Switch task and chat shortcuts', sessions: 1, tokens: 2_100_000, cost: 1.04, lastAt: hoursAgo(3) },
      { project: 'storefront', issue: 0, session: 'storefront/k3j2h4g5', title: 'Look into the flaky login test', sessions: 1, tokens: 6_700_000, cost: 3.31, lastAt: hoursAgo(26) },
      { project: 'agentos', issue: 309, session: '', title: '#309 Empty states for session groups', sessions: 3, tokens: 31_900_000, cost: 15.6, lastAt: hoursAgo(50) },
    ]
    const usage = { totals: usageRow('', '', 1, totals.sessions + totals.chats), tasks: usageRow('', '', 0.92, totals.sessions), byModel, pricesAsOf: '2026-10-06' }
    const weight = shape.reduce((a, d) => a + d.weight, 0) || 1
    const perDay = shape.map((d) => ({ day: d.day, prompts: Math.round((totals.prompts * d.weight) / weight), workMs: Math.round((totals.workMs * d.weight) / weight), cost: (totals.cost * d.weight) / weight }))
    const before = (n: number) => Math.round(n * 0.88)
    const previous: LedgerRow =
      days === 0
        ? row('', 0)
        : { ...totals, prompts: before(totals.prompts), sessions: before(totals.sessions * 1.1), workMs: before(totals.workMs * 0.9), cost: totals.cost * 0.85, prsMerged: before(totals.prsMerged) }
    return { days, since: dayKey(120), totals, projects, previous, perDay, usage, work: work.filter((w) => !project || w.project === project) }
  }

  function logCleanup(s: Session, removed: string[], reason: string) {
    const target = owner(s)
    target.cleanups.unshift({ at: Date.now(), sessionTitle: s.title, issue: s.issue, pr: s.pr?.number ?? 0, merged: s.pr?.state === 'merged', status: reason ? 'blocked' : 'done', removed, reason })
    if (target === current) emit('cleanups', { project: target.name, items: [...target.cleanups] })
  }

  // keptFor is why an unforced clean-up keeps the thread's worktree, for the reasons the mock can see, in the app's order.
  function keptFor(s: Session): string {
    if (!s.worktree) return ''
    const g = s.git
    if (g && g.uncommitted > 0) return `the worktree of ${s.branch} has uncommitted changes`
    if (g && s.pr?.state === 'merged' && g.notInMain > 0) return `${s.branch} has commits that are not in the merged pull request`
    if (g && s.pr?.state !== 'merged' && g.unpushed > 0) return g.unpushed === 1 ? `${s.branch} has a commit that is not on origin` : `${s.branch} has ${g.unpushed} commits that are not on origin`
    if (s.state !== 'settled' && chats.busy(s.id)) return 'a turn is running'
    return ''
  }

  // finishCleanup acts as the app's clean-up: the worktree, browser window and temp folder go, and the thread settles
  // and keeps its row and branch, with no PR left on it.
  function finishCleanup(s: Session) {
    const removed = [...(s.worktree ? [`worktree ${s.worktree}`] : []), 'thread put to sleep', ...(s.browser ? ['browser window'] : []), 'temp folder']
    logCleanup(s, removed, '')
    if (s.state !== 'settled') chats.end(s.id, 'settle')
    Object.assign(s, { worktree: '', pr: null, git: null, prAttention: '', cleanup: '', cleanupReason: '', browser: false })
    setState(s, 'settled')
    emit('issues', { project: current.name, items: currentIssues() })
  }

  // runningDescendant is a child of the thread, or a child of an asleep one, that runs, as a delete refuses.
  const runningDescendant = (id: string): Session | undefined => {
    for (const child of sessions.filter((x) => x.parent === id)) {
      const running = child.state === 'settled' ? runningDescendant(child.id) : child
      if (running) return running
    }
    return undefined
  }

  // worktreeUser is another thread that works in the thread's worktree, as far as the mock can tell: a fork of it, or a
  // child in its folder. The threads of a delete's family do not count.
  const worktreeUser = (s: Session, family: Session[]) =>
    sessions.find((x) => x.id !== s.id && !family.includes(x) && (x.forkOf === s.id || (x.parent === s.id && x.worktree === '')))

  // filesReason says why Edit from here cannot put the thread's files back, as the sessions do: a fork works in a
  // worktree agentos did not make for it, and another thread may work in the thread's own.
  const filesReason = (s: Session) => {
    if (s.forkOf) return 'Files are rewound only in a worktree agentos made for this thread'
    const user = worktreeUser(s, [])
    return user ? `Thread ${user.title} also works in this worktree` : ''
  }

  // descendants are the thread's children, their children and so on, nearest first.
  const descendants = (id: string): Session[] => {
    const out: Session[] = []
    for (let next = [id]; next.length > 0; ) {
      const children = sessions.filter((x) => next.includes(x.parent))
      out.push(...children)
      next = children.map((x) => x.id)
    }
    return out
  }

  // takenWorktrees are the threads of the thread s's family whose worktrees a delete of s may take, the deepest first.
  const takenWorktrees = (s: Session) => {
    const family = [...descendants(s.id).reverse(), s]
    return family.filter((x) => x.worktree && !worktreeUser(x, family))
  }

  const deletePlan = (s: Session): DeletePlan => {
    const taken = takenWorktrees(s)
    const children = descendants(s.id).filter((x) => x.state === 'settled').map((x) => x.title)
    return { worktrees: taken.length, children }
  }

  const merging = sessions.find((s) => s.cleanup === 'pending')
  if (merging) setTimeout(() => finishCleanup(merging), flags.done ? 1500 : 9000)

  const failing = sessions.find((s) => s.issue === 389)
  if (failing) {
    setTimeout(() => {
      failing.pr = pr(455, 'open', 'failing', { title: 'Retry webhooks with backoff', failedChecks: ['unit-tests'] })
      failing.prAttention = 'checks'
      publish()
      if (current.name === failing.project) emit('attention', { id: failing.id, state: 'pr' })
    }, 14000)
  }

  const pages = new Map<string, PageModel>()
  const evidence = new Map<string, Evidence[]>()
  const shotCanvas = document.createElement('canvas')
  let evidenceSeq = 0

  function browserStateOf(id: string): BrowserState {
    const page = pages.get(id)
    return {
      id,
      open: Boolean(page),
      url: page?.url ?? '',
      title: page?.title ?? '',
      loading: page?.loading ?? false,
      error: '',
      loadedAt: 0,
      pages: page ? 1 : 0,
      console: [],
    }
  }

  function pushBrowserState(id: string) {
    emit('browser:state', browserStateOf(id))
  }

  function navigate(id: string, url: string) {
    const page = pages.get(id)
    if (!page) throw 'This session has no browser open'
    page.url = url
    page.title = pageTitle(url)
    page.loading = true
    pushBrowserState(id)
    setTimeout(() => {
      page.loading = false
      pushBrowserState(id)
    }, 500)
  }

  function capture(id: string): string {
    const page = pages.get(id)
    if (!page) throw 'This session has no browser open'
    return renderPage(page, shotCanvas)
  }

  function addEvidence(s: Session, item: Omit<Evidence, 'id' | 'at'>, at = Date.now()): Evidence {
    const entry: Evidence = { ...item, id: `ev-${++evidenceSeq}`, at }
    const items = [...(evidence.get(s.id) ?? []), entry]
    evidence.set(s.id, items)
    s.evidence = items.length
    publish()
    emit('evidence', { id: s.id, items: items.map((e) => ({ ...e })) })
    if (item.source === 'agent') emit('attention', { id: s.id, state: 'evidence' })
    return entry
  }

  function openPage(s: Session, url: string): PageModel {
    const page = newPage(url)
    pages.set(s.id, page)
    s.browser = true
    publish()
    pushBrowserState(s.id)
    return page
  }

  const driven = sessions.find((s) => s.issue === 454)
  if (driven) {
    const page = openPage(driven, 'https://shop.acme.test/products')
    const steps: (() => void)[] = [
      () => navigate(driven.id, 'https://shop.acme.test/products'),
      () => {
        const { field } = pageRects()
        clickAt(page, field.x + 10, field.y + 10)
        typeText(page, 'canvas tote')
      },
      () => {
        const { button } = pageRects()
        clickAt(page, button.x + 10, button.y + 10)
      },
      () => addEvidence(driven, { kind: 'image', url: capture(driven.id), text: '', caption: 'Search results for "canvas tote" after the page split', source: 'agent' }),
      () => {
        page.typed = ''
        page.submitted = ''
        navigate(driven.id, 'https://shop.acme.test/cart')
      },
    ]
    let step = 0
    setTimeout(() => {
      setInterval(() => steps[step++ % steps.length](), 4500)
    }, 6000)
    addEvidence(driven, { kind: 'image', url: capture(driven.id), text: '', caption: 'Before: products list on main', source: 'agent' }, Date.now() - 50 * minute)
    page.submitted = 'tote'
    addEvidence(driven, { kind: 'image', url: capture(driven.id), text: '', caption: 'After: same page on issue-454', source: 'agent' }, Date.now() - 44 * minute)
    page.submitted = ''
    addEvidence(driven, { kind: 'text', url: '', text: 'tsc --noEmit: 0 errors\nvitest: 214 passed, 0 failed\nThe gallery now loads on the client only.', caption: 'Checks before the PR', source: 'agent' }, Date.now() - 20 * minute)
  }
  const second = sessions.find((s) => s.issue === 326)
  if (second) addEvidence(second, { kind: 'text', url: '', text: 'Totals are summed in cents and formatted once. Rounding each line drifted by a cent on large carts.', caption: 'Why cents', source: 'agent' }, Date.now() - 30 * minute)

  const seedDigest = (): DigestItem[] => {
    const now = Date.now()
    const item = (n: number, at: number, source: string, title: string, why: string): DigestItem => ({
      id: `dg-${n}`, title, why, url: `https://example.com/changelog/${n}`, source, at, noteId: '',
    })
    return [
      item(1, now - 2 * 86_400_000, 'Claude Code', 'Hooks can match Bash commands with a pattern', 'Could replace the per-project permission rules in the config.'),
      item(2, now - 2 * 86_400_000, 'wails', 'v2.11 fixes the drag region in full screen', 'The hidden title bar here uses the drag region.'),
      item(3, now - 2 * 86_400_000, 'react', 'ReactDOM.render warnings are louder in 18.3', 'Two entry files still call render.'),
      item(4, now - 2 * 86_400_000, 'tailwind', 'v4.1 ships container queries in core', 'Could unblock the product grid layout work.'),
      item(5, now - 2 * 86_400_000, 'drizzle', 'Batch inserts land in 0.40', 'The customer import inserts rows one by one.'),
      item(6, now - 9 * 86_400_000, 'Claude Code', 'Plan mode now shows a checklist', 'Matches how /investigate presents its plan.'),
      item(7, now - 9 * 86_400_000, 'bullmq', 'Flow producers get a retry option', 'The image pipeline retries by hand today.'),
    ]
  }
  const lastDigestRun = Date.now() - 2 * 86_400_000
  let digest: Digest = { running: false, startedAt: lastDigestRun - 124_000, tools: 23, lastRunAt: lastDigestRun, nextRunAt: lastDigestRun + 7 * 86_400_000, error: '', stoppedAt: 0, project: data[0].name, items: seedDigest() }
  const pushDigest = () => emit('digest', { ...digest, items: digest.items.map((i) => ({ ...i })) })

  const delay = <T,>(value: T, ms = 220) => new Promise<T>((resolve) => setTimeout(() => resolve(value), ms))

  // With ?offline, GitHub calls get no answer for the first 12 seconds, as when the Mac has no network.
  const networkBackAt = flags.offline ? Date.now() + 12_000 : 0
  let offline = false
  const github = async <T,>(value: T, ms: number): Promise<T> => {
    await delay(undefined, ms)
    if (Date.now() < networkBackAt) {
      offline = true
      emit('network', { online: false })
      throw new Error('error connecting to api.github.com')
    }
    if (offline) {
      offline = false
      emit('network', { online: true })
    }
    return value
  }
  // The app's first gh calls, such as the PR fetch, start when it does.
  if (flags.offline) github(undefined, 1000).catch(() => {})

  let settings: Settings = { theme: 'system', darkTheme: 'black', lightTheme: 'light', textScale: 1, keepAwake: true, cleanup: { merge: 'auto', close: 'manual' }, promptSend: 'auto', browserEnabled: true, digestSchedule: 'weekly', threadPlace: '', threadAccess: '', dataDir: '~/.local/share/agentos', dataDirFixed: false }

  const issueRefreshes = { count: 0 }
  Object.assign(window, { issueRefreshes })

  const backend = {
    Snapshot: async () => snapshot(),
    Awake: async () => data.some((p) => projectView(p).working > 0),
    NewThread: async (title: string, prompt: string, model: string, effort: string, access: Access, place: Place | '') => {
      if (access && !['read', 'supervised', 'full'].includes(access)) throw `access must be read, supervised or full, not "${access}"`
      if (place && !['worktree', 'project'].includes(place)) throw `place must be worktree or project, not "${place}"`
      const name = title.trim() || titleFromPrompt(prompt) || 'agent'
      const acc = access || 'supervised'
      if ((place || settings.threadPlace || 'worktree') === 'project') return startSessionFor(name, 0, prompt, '', model, effort, acc)
      const branch = freeBranch(branchFromTitle(name), (b) => sessions.some((x) => x.branch === b))
      return startSessionFor(name, 0, prompt, '', model, effort, acc, { branch, worktree: `${current.dir}/trees/${branch}` })
    },
    // DeleteThread acts as the sessions do: with worktree, the thread's own worktree goes first through a clean-up
    // with the checks, and one they keep refuses the delete; the thread, its chat and its row then go.
    DeleteThread: async (id: string, worktree: boolean) => {
      if (deleted.has(id)) throw `Thread ${id} was deleted`
      const s = find(id)
      if (s.cleanup === 'pending') throw `Thread ${s.n} is being cleaned up; delete it when that ends`
      const child = runningDescendant(id)
      if (child) throw `Thread ${s.n} has a running child, ${child.title}; stop it first`
      const family = [...descendants(id).reverse(), s]
      const busy = family.find((x) => x.cleanup === 'pending')
      if (busy) throw `Thread ${busy.n} is being cleaned up; delete it when that ends`
      const user = worktree && s.worktree ? worktreeUser(s, family) : undefined
      if (user) throw `Thread ${s.n} keeps its worktree: Thread ${user.title} also works in this worktree. Delete it without the worktree`
      const taken = worktree ? takenWorktrees(s) : []
      for (const x of taken) {
        const reason = keptFor(x)
        if (!reason) continue
        Object.assign(x, { cleanup: 'blocked', cleanupReason: reason })
        logCleanup(x, [], reason)
        publish()
        throw `Thread ${x.n} keeps its worktree: ${reason}. Untick the box to keep ${taken.length > 1 ? 'the worktrees' : 'the worktree'}, or clean Thread ${x.n} up first`
      }
      for (const x of taken) logCleanup(x, [`worktree ${x.worktree}`, 'thread put to sleep', ...(x.browser ? ['browser window'] : []), 'temp folder'], '')
      for (const x of family) {
        if (x.state !== 'settled') chats.end(x.id, 'settle')
        deleted.add(x.id)
        servers.delete(x.id)
        sessions.splice(sessions.indexOf(x), 1)
      }
      publish()
      emit('issues', { project: current.name, items: currentIssues() })
    },
    DeletePlan: async (id: string) => deletePlan(find(id)),
    Servers: async (id: string) => [...(servers.get(find(id).id) ?? [])].sort((a, b) => a.port - b.port),
    StopServer: async (id: string, pid: number) => {
      const list = servers.get(find(id).id) ?? []
      if (!list.some((server) => server.pid === pid)) throw 'That server is not running'
      servers.set(id, list.filter((server) => server.pid !== pid))
    },
    OpenServer: async (port: number) => {
      if (!Number.isInteger(port) || port < 1 || port > 65535) throw `${port} is not a port`
      window.open(`http://localhost:${port}`, '_blank')
    },
    WakeThread: async (id: string) => {
      wake(find(id))
      return view(find(id))
    },
    ForkSession: async (id: string, at: string) => {
      const from = find(id)
      if (!from.hosted) throw 'Only a thread with a chat can be forked'
      const s = create(`${from.title} (fork)`, from.issue, 'idle', [], Date.now(), current, from.access)
      // Like the real fork: no worktree, and the branch only the issue's fallback that create gives it.
      Object.assign(s, { model: from.model, effort: from.effort, action: '', forkOf: from.id, worktree: '' })
      try {
        chats.fork(from.id, s.id, at)
      } catch (err) {
        sessions.splice(sessions.indexOf(s), 1)
        throw err
      }
      chats.start(s.id)
      publish()
      return view(s)
    },
    SettleThread: async (id: string) => {
      const s = find(id)
      if (s.state === 'settled') throw `Thread ${id} is asleep`
      chats.end(id, 'settle')
      servers.delete(id)
      setState(s, 'settled')
      emit('issues', { project: current.name, items: currentIssues() })
    },
    // SetAccess acts as the sessions do: a settled thread wakes with it, Read and Supervised change as Claude runs, and
    // a change to or from Full starts the thread again. The thread's chat, not its row, says whether a turn is open: a
    // thread waiting on a reply that asks has none.
    SetAccess: async (id: string, access: Access) => {
      if (!access) throw 'Pick an access level'
      if (!['read', 'supervised', 'full'].includes(access)) throw `access must be read, supervised or full, not "${access}"`
      const s = find(id)
      if (!s.hosted) throw `Thread ${id} ran in a terminal; its access cannot change`
      if (s.state !== 'settled' && chats.busy(id)) throw s.state === 'working' && s.subagents > 0 && !chats.turnRuns(id) ? 'Change access when the subagents end' : 'Change access when the turn ends'
      if (s.access === access) return
      if (s.state !== 'settled' && (s.access === 'full' || access === 'full')) {
        chats.end(id, { restart: access })
        chats.start(id)
      } else if (s.state !== 'settled') chats.mode(id, modeOfAccess[access])
      s.access = access
      publish()
    },
    RenameSession: async (id: string, title: string) => {
      find(id).title = title
      publish()
    },
    SwitchProject: async (name: string) => {
      const next = data.find((p) => p.name === name)
      if (!next) throw 'No such project'
      return switchTo(next)
    },
    AddProject: async () => {
      const name = `picked-folder-${data.length}`
      data.push({ name, dir: `~/code/${name}`, repo: '', notes: [], issues: [], waits: [], cleanups: [], needsSetup: true })
      const snap = switchTo(data[data.length - 1])
      emit('projects', snap.projects)
      return snap
    },
    AddProjectDir: async (dir: string) => {
      if (!/^[~/]/.test(dir) || dir.includes('nope')) throw `Folder does not exist: ${dir}`
      const name = dir.replace(/\/+$/, '').split('/').pop() || dir
      if (data.some((p) => p.name === name)) throw `Project ${name} already exists`
      data.push({ name, dir, repo: '', notes: [], issues: [], waits: [], cleanups: [], needsSetup: true })
      const snap = switchTo(data[data.length - 1])
      emit('projects', snap.projects)
      return snap
    },
    NewProject: async (name: string) => {
      if (!/^[\w.-]+$/.test(name)) throw `${name} is not a project name`
      if (data.some((p) => p.name === name)) throw `Project ${name} already exists`
      data.push({ name, dir: `~/code/${name}`, repo: '', notes: [], issues: [], waits: [], cleanups: [], needsSetup: true })
      const snap = switchTo(data[data.length - 1])
      emit('projects', snap.projects)
      return snap
    },
    CreateRepo: async (name: string) => {
      const p = data.find((p) => p.name === name)
      if (!p) throw `no project called ${name}`
      if (name.includes('taken')) throw `${name} has no GitHub repo: name already exists`
      p.repo = `ned/${name}`
      return snapshot()
    },
    RemoveProject: async (name: string) => {
      const at = data.findIndex((p) => p.name === name)
      if (at < 0) throw 'No such project'
      if (data.length === 1) throw 'Cannot remove the last project'
      data.splice(at, 1)
      const snap = current.name === name ? switchTo(data[0]) : snapshot()
      emit('projects', snap.projects)
      return snap
    },
    SetUpProject: async () => {
      if (current.needsSetup) current.issues = current.issues.map((issue) => ({ ...issue, section: 'Inbox', actions: ['Work'], access: actionAccess('Work'), moves: movesFrom('Inbox') }))
      current.needsSetup = false
      emit('projects', data.map(projectView))
      return startSessionFor('agentos setup', 0, 'Set up this project for agentos with me.')
    },
    DismissSetup: async () => {
      current.setupDismissed = true
      const snap = snapshot()
      emit('projects', snap.projects)
      return snap
    },
    AddNote: async (text: string) => {
      const now = Date.now()
      const note: Note = { id: `note-${Math.random().toString(36).slice(2, 8)}`, text, createdAt: now, updatedAt: now, pinned: false, archived: false, images: [] }
      current.notes.push(note)
      touchNotes()
      return { ...note }
    },
    UpdateNote: async (id: string, text: string) => {
      const note = noteById(id)
      note.text = text
      note.updatedAt = Date.now()
      touchNotes()
      return { ...note }
    },
    SetNotePinned: async (id: string, pinned: boolean) => {
      const note = noteById(id)
      note.pinned = pinned
      touchNotes()
      return { ...note }
    },
    SetNoteArchived: async (id: string, archived: boolean) => {
      const note = noteById(id)
      note.archived = archived
      if (archived) note.pinned = false
      touchNotes()
      return { ...note }
    },
    AddNoteImage: async (id: string, base64: string, mime: string) => {
      const note = noteById(id)
      note.images = [...note.images, `data:${mime};base64,${base64}`]
      touchNotes()
      return { ...note }
    },
    RemoveNoteImage: async (id: string, url: string) => {
      const note = noteById(id)
      note.images = note.images.filter((u) => u !== url)
      touchNotes()
      return { ...note }
    },
    DeleteNote: async (id: string) => {
      noteById(id)
      current.notes = current.notes.filter((n) => n.id !== id)
      touchNotes()
    },
    NoteToIssue: async (id: string) => {
      const note = noteById(id)
      if (!current.repo) throw 'This project has no GitHub repo'
      const number = nextIssue++
      const now = Date.now()
      current.issues.unshift({
        number,
        title: note.text.split('\n')[0].slice(0, 90),
        type: '',
        section: 'Inbox',
        actions: ['Plan', 'Investigate', 'Work'],
        access: actionAccess('Plan'),
        moves: movesFrom('Inbox'),
        url: `https://github.com/${current.repo}/issues/${number}`,
        sessionId: '',
        author: 'nednella',
        assignees: [],
        labels: [],
        ignored: [],
        createdAt: now,
        updatedAt: now,
      })
      current.notes = current.notes.filter((n) => n.id !== id)
      touchNotes()
      emit('issues', { project: current.name, items: currentIssues() })
    },
    NoteToSession: async (id: string) => {
      const note = noteById(id)
      return startSessionFor(note.text.split('\n')[0], 0, note.text)
    },
    Issues: async (refresh: boolean) => {
      if (refresh) issueRefreshes.count++
      return refresh ? github(currentIssues(), 700) : delay(currentIssues())
    },
    StartIssue: async (number: number, action: string, prompt: string, model: string, effort: string) => {
      const issue = current.issues.find((i) => i.number === number)
      if (!issue) throw 'No such issue'
      const mine = sessions.filter((s) => s.project === current.name && !s.forkOf && s.issue === number)
      const live = mine.find((s) => s.state !== 'settled')
      if (live) return view(live)
      if (action && !issue.actions.includes(action)) throw `issue #${number} has no action "${action}"`
      const name = action || issue.actions[0]
      const command = { Work: `/work ${number}`, Plan: `/plan ${number}`, Investigate: `/investigate ${number}` }[name] ?? `Work on issue #${number}: ${issue.title}`
      const settled = mine.filter((s) => s.hosted).at(-1)
      if (settled) {
        wake(settled)
        void chats.backend.ChatSend(settled.id, `${command} ${prompt}`.trim(), [])
        return view(settled)
      }
      const short = issue.title.split(' ').slice(0, 3).join(' ')
      const session = startSessionFor(`#${number} ${short}`, number, `${command} ${prompt}`.trim(), name, model, effort, actionAccess(name))
      emit('issues', { project: current.name, items: currentIssues() })
      return session
    },
    MoveIssue: async (number: number, section: string) => {
      const issue = current.issues.find((i) => i.number === number)
      if (!issue) throw 'No such issue'
      const moved = moveIssue(issue, section)
      await delay(null, 500)
      lastMoves.set(`${current.name}#${number}`, issue)
      current.issues[current.issues.indexOf(issue)] = moved
      sortBySection(current.issues)
      emit('issues', { project: current.name, items: currentIssues() })
    },
    UndoMove: async (number: number) => {
      const before = lastMoves.get(`${current.name}#${number}`)
      const at = current.issues.findIndex((i) => i.number === number)
      if (!before || at < 0) throw `nothing to undo for issue #${number}`
      await delay(null, 500)
      lastMoves.delete(`${current.name}#${number}`)
      current.issues[at] = before
      sortBySection(current.issues)
      emit('issues', { project: current.name, items: currentIssues() })
    },
    IssueDetail: async (number: number) => {
      if (!current.issues.some((i) => i.number === number)) throw `reading issue #${number}: gh api: HTTP 404: Not Found`
      return delay(buildIssueDetail(number), 350)
    },
    ShellOpen: async (sessionId: string) => {
      return { id: (shellsOf(current.name)[0] ?? startShell(sessionId)).id }
    },
    ShellNew: async (sessionId: string) => ({ id: startShell(sessionId).id }),
    ShellClose: async (id: string) => {
      const at = shells.findIndex((x) => x.id === id)
      if (at >= 0) shells.splice(at, 1)
    },
    TermOpen: async (id: string) => {
      const shell = findShell(id)
      if (!shell) throw 'Only a shell has a terminal'
      shell.opened = true
      emit('term:data', { id, data: toBase64(`\x1bc${shell.buffer}`) })
    },
    TermWrite: async (id: string, data: string) => {
      const shell = findShell(id)
      if (!shell) return
      for (const ch of data) {
        if (ch === '\r') {
          const line = shell.line
          shell.line = ''
          shellRun(shell, line)
          shellWrite(shell, `\r\n${prompt(shell.project)}`)
        } else if (ch === '\x7f') {
          if (shell.line) {
            shell.line = shell.line.slice(0, -1)
            shellWrite(shell, '\b \b')
          }
        } else if (ch >= ' ') {
          shell.line += ch
          shellWrite(shell, ch)
        }
      }
    },
    TermResize: async () => undefined,
    TermClose: async (id: string) => {
      const shell = findShell(id)
      if (shell) shell.opened = false
    },
    Stats: async (days: number, project: string) => delay(statsFor(days, project), 150),
    Ledger: async (days: number, project: string) => delay(ledgerFor(days, project), 300),
    RefreshPRs: async () => github(undefined, 500),
    AckPR: async (id: string) => {
      const s = find(id)
      s.prAttention = ''
      if (s.pr) s.pr.newComments = 0
      publish()
    },
    // A thread with a branch and no worktree lost its worktree to a clean-up; Go does not fall back to the project folder.
    OpenInEditor: async (id: string) => {
      const s = find(id)
      if (s.branch !== '' && s.worktree === '') throw "This thread's folder is gone"
    },
    TypeInto: async (id: string, text: string) => {
      const s = find(id)
      if (!s.hosted) throw `Thread ${id} runs in an old terminal`
      if (s.state === 'settled') throw `Thread ${id} is asleep`
      return chats.backend.ChatDraft(id, text)
    },
    ...chats.backend,
    Cleanup: async (id: string, force: boolean) => {
      const s = find(id)
      if (s.cleanup === 'pending') throw 'clean-up is already running'
      const reason = force ? '' : keptFor(s)
      if (!reason) return finishCleanup(s)
      Object.assign(s, { cleanup: 'blocked', cleanupReason: reason })
      logCleanup(s, [], reason)
      publish()
    },
    Cleanups: async () => [...current.cleanups],
    Memory: async () => memory(),
    // As if Claude's settings held model opus, effortLevel high, and modelSettings.haiku.effortLevel low.
    DefaultModel: async (model: string) => {
      if (flags.noModel) return { model, effort: '' }
      const picked = model || 'opus'
      return { model: picked, effort: picked === 'haiku' ? 'low' : 'high' }
    },
    BrowserOpen: async (id: string, url: string) => {
      const s = find(id)
      openPage(s, url ? normalizeUrl(url) : 'https://docs.acme.test/')
      return browserStateOf(id)
    },
    BrowserGoto: async (id: string, url: string) => navigate(id, normalizeUrl(url)),
    BrowserShow: async () => {},
    BrowserState: async (id: string) => browserStateOf(id),
    BrowserScreenshot: async (id: string, caption: string) =>
      addEvidence(find(id), { kind: 'image', url: capture(id), text: '', caption, source: 'user' }),
    BrowserClose: async (id: string) => {
      const s = find(id)
      pages.delete(id)
      s.browser = false
      publish()
      pushBrowserState(id)
    },
    Evidence: async (id: string) => (evidence.get(id) ?? []).map((e) => ({ ...e })),
    DeleteEvidence: async (id: string, evidenceId: string) => {
      const s = find(id)
      const items = (evidence.get(id) ?? []).filter((e) => e.id !== evidenceId)
      evidence.set(id, items)
      s.evidence = items.length
      publish()
      emit('evidence', { id, items })
    },
    Digest: async () => ({ ...digest, items: digest.items.map((i) => ({ ...i })) }),
    RunDigest: async () => {
      if (digest.running) throw 'A digest run is already in progress'
      const startedAt = Date.now()
      digest = { ...digest, running: true, startedAt, error: '', stoppedAt: 0 }
      pushDigest()
      // The run counts the project's tools in the background, after it has started.
      setTimeout(() => {
        digest = { ...digest, tools: 23 }
        pushDigest()
      }, 100)
      // As the real run, each item arrives as the agent hands it in, after the run's earlier items.
      const fresh = [
        ['Claude Code', 'Skills can declare allowed tools', 'Lets /work run without a permission prompt for tsc.', 'https://code.claude.com/docs/changelog'],
        ['wails', 'Native file drop events', 'Notes could take dropped screenshots from Finder.', 'https://wails.io/changelog'],
        ['xterm.js', 'Synchronized output mode', 'Would stop flicker while the agent redraws.', 'https://github.com/xtermjs/xterm.js/releases'],
        ['vite', 'v8 build is faster', 'No change needed, nice to have for the shop.', 'https://vite.dev/blog'],
        ['gh', 'pr checks --watch exits on first failure', 'The PR poller could use it.', 'https://github.com/cli/cli/releases'],
      ]
      let added = 0
      fresh.forEach(([source, title, why, url], i) =>
        setTimeout(() => {
          if (digest.items.some((item) => item.url === url)) return
          const item: DigestItem = { id: `dg-new-${startedAt}-${i}`, title, why, url, source, at: Date.now(), noteId: '' }
          digest = { ...digest, items: [...digest.items.slice(0, added), item, ...digest.items.slice(added)].slice(0, 30) }
          added++
          pushDigest()
        }, 700 * (i + 1)),
      )
      setTimeout(() => {
        const now = Date.now()
        digest = { ...digest, running: false, lastRunAt: now, nextRunAt: now + 7 * 86_400_000 }
        pushDigest()
      }, 700 * (fresh.length + 1))
    },
    DigestToNote: async (itemId: string) => {
      const item = digest.items.find((i) => i.id === itemId)
      if (!item) throw 'No such digest item'
      const now = Date.now()
      const note: Note = { id: `note-${Math.random().toString(36).slice(2, 8)}`, text: `${item.title}\n${item.why}\n${item.url}`, createdAt: now, updatedAt: now, pinned: false, archived: false, images: [] }
      current.notes.push(note)
      item.noteId = note.id
      touchNotes()
      pushDigest()
      return { ...note }
    },
    DismissDigestItem: async (itemId: string) => {
      digest = { ...digest, items: digest.items.filter((i) => i.id !== itemId) }
      pushDigest()
    },
    Settings: async () => settings,
    SetTheme: async (theme: ThemeSetting) => {
      settings = { ...settings, theme }
      return settings
    },
    SetThemes: async (darkTheme: DarkTheme, lightTheme: LightTheme) => {
      settings = { ...settings, darkTheme, lightTheme }
      return settings
    },
    SetCleanup: async (event: CleanupEvent, mode: CleanupMode) => {
      settings = { ...settings, cleanup: { ...settings.cleanup, [event]: mode } }
      return settings
    },
    SetTextScale: async (textScale: number) => {
      settings = { ...settings, textScale }
      return settings
    },
    SetKeepAwake: async (keepAwake: boolean) => {
      settings = { ...settings, keepAwake }
      return settings
    },
    SetPromptSend: async (promptSend: PromptSend) => {
      settings = { ...settings, promptSend }
      return settings
    },
    SetBrowserEnabled: async (browserEnabled: boolean) => {
      settings = { ...settings, browserEnabled }
      return settings
    },
    SetDigestSchedule: async (digestSchedule: DigestSchedule) => {
      settings = { ...settings, digestSchedule }
      return settings
    },
    SetThreadPlace: async (threadPlace: Place | '') => {
      if (threadPlace && !['worktree', 'project'].includes(threadPlace)) throw `place must be worktree or project, not "${threadPlace}"`
      settings = { ...settings, threadPlace }
      return settings
    },
    SetThreadAccess: async (threadAccess: Access | '') => {
      if (threadAccess && !['read', 'supervised', 'full'].includes(threadAccess)) throw `access must be read, supervised or full, not "${threadAccess}"`
      settings = { ...settings, threadAccess }
      return settings
    },
    PickDataDir: async () => delay({ dir: '~/Library/Mobile Documents/com~apple~CloudDocs/agentos', empty: !flags.fullDataDir }),
    SetDataDir: async (_dir: string, withData: boolean) => {
      await delay(undefined, withData ? 2500 : 400)
    },
    Update: async () => {
      await delay(undefined, 2500)
      throw 'The mock cannot relaunch'
    },
    PatchNotes: async () => (flags.patchNotes ? PATCH_NOTES : []),
    Changelog: async () => PATCH_NOTES,
    OpenURL: async (url: string) => {
      window.open(url, '_blank')
    },
  }

  return { backend, on, flags, mediaURL: chats.mediaURL }
}
