import type { Access, ChatEvent, ChatImage, EventMap, FileContent, FileEntry, FileListing, FilesRewind, Pending, State } from './types'

type Emit = <K extends keyof EventMap>(event: K, payload: EventMap[K]) => void

// What the mock knows of a thread: whether it has settled, its worktree, '' for a thread in the project folder, a way
// to move its row's state as hooks would, a way to wake it when it has settled, and ways to follow the permission mode
// its Claude reports and the subagents it runs in the background, as the threads service does.
export type MockThread = { n: number; settled: boolean; worktree: string; filesReason(): string; setState(state: State): void; wake(): void; followMode(mode: string): void; setSubagents(n: number): void }

type Draft = Omit<ChatEvent, 'seq' | 'at'> & { at?: number }

// MockChild is a child a mock thread delegated to: its task, and its result once it has one. project is set for a child
// in another project.
export type MockChild = { id: string; title: string; task: string; access: Access; project?: string; result?: string }

const minute = 60_000
const batchEvery = 33 // the service sends a thread's events at most this often, in ms
// As the service does: at most this many images to a message, each at most host.MaxImage bytes.
const maxImages = 10
const maxImage = 3932160
const imageExts: Record<string, string> = { 'image/png': '.png', 'image/jpeg': '.jpg', 'image/gif': '.gif', 'image/webp': '.webp' }
const commands = [
  { name: 'review', description: 'Review a pull request', argumentHint: '<pr>', builtin: true },
  { name: 'compact', description: 'Clear the conversation but keep a summary of it', argumentHint: '<instructions>', builtin: true },
  { name: 'greet', description: 'Say a fixed greeting (project)', argumentHint: '' },
]
// The files of every mock thread's folder, as git lists them.
const files = [
  '.gitignore',
  'AGENTS.md',
  'README.md',
  'docs/config.md',
  'docs/testing.md',
  'package.json',
  'app-frontend/package.json',
  'app-frontend/src/js/routes.tsx',
  'app-frontend/src/js/cart.ts',
  'app-frontend/src/js/money.ts',
  'app-frontend/src/js/pages/Cart.tsx',
  'app-frontend/src/js/pages/Checkout.tsx',
  'app-frontend/src/js/pages/checkout_steps.tsx',
  'app-frontend/src/js/pages/[slug]/Index.tsx',
  'app-frontend/src/js/hooks/useCart.ts',
  'app-frontend/src/js/hooks/useCheckout.ts',
  'app-frontend/test/checkout.test.ts',
  'app-backend/cmd/server/main.go',
  'app-backend/internal/cart/cart.go',
  'app-backend/internal/cart/cart_test.go',
  'app-frontend/public/logo.svg',
  'app-backend/server.log',
  'app-backend/data.bin',
]
const fileLimit = 30
// What git ignores: a file and a folder it lists whole, and what the folders hold.
const ignoredRoot = ['.env', 'node_modules']
const ignoredFolders: Record<string, string[]> = {
  node_modules: ['node_modules/left-pad', 'node_modules/react', 'node_modules/.package-lock.json'],
  'node_modules/left-pad': ['node_modules/left-pad/index.js', 'node_modules/left-pad/package.json'],
  'node_modules/react': ['node_modules/react/index.js'],
}
const source: Record<string, string> = {
  '.gitignore': 'node_modules/\n.env\n*.log\n',
  '.env': 'STRIPE_KEY=sk_test_demo\n',
  'package.json': '{\n  "name": "storefront",\n  "private": true,\n  "scripts": {\n    "test": "npm --prefix app-frontend test"\n  }\n}\n',
  'app-frontend/src/js/cart.ts':
    "import { money } from './money'\n\nexport type Line = { sku: string; quantity: number; price: number }\n\n// total is what the cart costs, rounded as the config says.\nexport function total(lines: Line[]): string {\n  const sum = lines.reduce((all, line) => all + line.quantity * line.price, 0)\n  return money(sum)\n}\n",
  'app-frontend/src/js/money.ts': 'export const money = (cents: number) => `$${(cents / 100).toFixed(2)}`\n',
  'app-backend/cmd/server/main.go': 'package main\n\nimport (\n\t"log"\n\t"net/http"\n)\n\nfunc main() {\n\tlog.Fatal(http.ListenAndServe(":8080", nil))\n}\n',
  'node_modules/left-pad/index.js': 'module.exports = (s, n) => String(s).padStart(n)\n',
}
const logo =
  '<svg xmlns="http://www.w3.org/2000/svg" width="240" height="120" viewBox="0 0 240 120"><rect width="240" height="120" rx="12" fill="#5b6ee1"/><text x="120" y="70" font-family="sans-serif" font-size="30" text-anchor="middle" fill="#fff">storefront</text></svg>'
const maxText = 1 << 20
// The text of the markdown files among them.
const markdown: Record<string, string> = {
  'AGENTS.md': '# AGENTS.md\n\nstorefront: the shop\'s web app and its API.\n\n## Hard rules\n\n- Tests run against a throwaway database, never the shared one.\n- Never push to `main`; one issue, one branch, one draft PR.\n\n## Commands\n\n| What | Command |\n| --- | --- |\n| Front end tests | `npm test` |\n| API tests | `go test ./...` |\n\nThe settings are in [docs/config.md](docs/config.md).',
  'README.md': '# storefront\n\nThe shop\'s web app (`app-frontend`) and its API (`app-backend`).\n\nSee [AGENTS.md](AGENTS.md) for how to work on it.',
  'docs/config.md': '# Config\n\nstorefront reads `config.yaml` at start.\n\n## Checkout\n\n- `checkout_steps`: the steps of the checkout, in order\n- `cart_rounding`: how the cart rounds a total, `half-up` or `bankers`\n\n| Key | Default |\n| --- | --- |\n| checkout_steps | address, payment, review |\n| cart_rounding | half-up |\n\n```yaml\ncheckout_steps:\n  - address\n  - payment\n  - review\n```\n\nHow to test a change to these is in [testing.md](testing.md).',
  'docs/testing.md': '# Testing\n\nRun `npm test` in `app-frontend` and `go test ./...` in `app-backend`.\n\nThe checkout tests start a fake payment provider on port 18080.',
}

// subsequence says whether s has the letters of sub in order, not always side by side.
function subsequence(s: string, sub: string) {
  let at = 0
  for (const c of sub) {
    at = s.indexOf(c, at)
    if (at < 0) return false
    at += c.length
  }
  return true
}

// rankFiles orders the files that match as the service does: file name prefix, then contains, then the letters in
// order, shorter first among equals.
function rankFiles(paths: string[], query: string): string[] {
  const q = query.toLowerCase()
  const rankOf = (lower: string) => (lower.slice(lower.lastIndexOf('/') + 1).startsWith(q) ? 0 : lower.includes(q) ? 1 : subsequence(lower, q) ? 2 : -1)
  return paths
    .map((path) => ({ path, rank: rankOf(path.toLowerCase()) }))
    .filter((f) => f.rank >= 0)
    .sort((a, b) => a.rank - b.rank || a.path.length - b.path.length)
    .slice(0, fileLimit)
    .map((f) => f.path)
}

const replies = [
  'Done. The checkout route now loads its own data, and the cart page no longer calls the cart API twice.',
  'I read the router and the checkout page. Nothing else needs to change for this.',
  'Tests pass: 48 passed, 0 failed.',
]

// thoughts are the thinking Claude streams before each reply, one per reply, as summaries Claude Code shows.
const thoughts = [
  "**Checking the checkout route**\n\nThe router lists the cart and the checkout. The checkout page fetched the cart again on load, which is the double call.",
  'I need to see whether anything else imports the old checkout data. The router and the page are the only two places, so nothing else moves.',
  "**Reading the test run**\n\nAll 48 pass and none were skipped, so the change holds.",
]

// toolCalls are the calls Claude makes before it replies to a message that says "tools", each running a while.
const toolRuns = 1500
const toolCalls = [
  { tool: 'Read', input: { file_path: 'app-frontend/src/js/routes.tsx' }, result: "export const routes = [\n  { path: '/cart', page: Cart },\n]" },
  { tool: 'Grep', input: { pattern: 'useCart' }, result: 'app-frontend/src/js/pages/Cart.tsx\napp-frontend/src/js/pages/Checkout.tsx' },
  { tool: 'Bash', input: { command: 'npm test', description: 'Run the tests' }, result: 'Tests: 48 passed, 0 failed' },
]

const olderReplies = [
  'The cart total rounds each line before it adds them up, so two lines of 0.005 come to 0.02 where the order says 0.01.',
  'Two places read the locale:\n\n- `formatPrice` in `money.ts`, which takes it from the store\n- `Checkout.tsx`, which reads `navigator.language` itself\n\nThe second one is the odd one out.',
  "Here is the retry loop as it stands:\n\n```ts\nfor (let i = 0; i < 3; i++) {\n  const res = await fetch(url)\n  if (res.ok) return res\n  await sleep(500 * 2 ** i)\n}\nthrow new Error('gave up')\n```\n\nIt waits 0.5 s, 1 s, then 2 s.",
  'Yes.',
  '## What changed\n\n1. The product page loads its sections in parallel.\n2. The reviews section waits for the fold.\n3. The gallery keeps its own state.\n\nNothing in the cart moved.',
]

const askedQuestions = [
  {
    question: 'Should I keep the old cart hook or replace it?',
    header: 'Cart hook',
    options: [
      { label: 'Keep it', description: 'Remove the new hook and leave useCart as it is.' },
      { label: 'Replace it', description: 'Move every caller to the new hook and delete useCart.' },
    ],
    multiSelect: false,
  },
  {
    question: 'Which pages should I check once it is fixed?',
    header: 'Pages',
    options: [
      { label: 'Cart', description: '' },
      { label: 'Checkout', description: '' },
      { label: 'Product', description: 'The add-to-cart button reads the cart too.' },
    ],
    multiSelect: true,
  },
]

const askedPlan = '## Split checkout into three steps\n\n1. Add `/checkout/address`, `/checkout/payment` and `/checkout/review` routes.\n2. Move the form state into a `useCheckout` store so each step reads it.\n3. Keep the cart page as it is.\n\nThe tests for `Checkout.tsx` move with the steps.'

// longReply is about 30 KB of headings, prose, lists, code and tables, streamed in deltas of up to 15 characters as
// often as Claude sends them.
const longDeltaEvery = 21
const longReply = Array.from({ length: 19 }, (_, i) => {
  const n = i + 1
  return [
    `## ${n}. The cart total, step ${n}`,
    `The cart adds up each line's **rounded price**, so two lines of \`0.005\` come to \`0.02\` where the order says *0.01*. Step ${n} reads the total from \`cart.ts\` and checks it against the order the server sent, line by line, before the owner pays. When the two differ, it shows the order's figure and logs the cart's, so the support team can see where the cent went.`,
    `What step ${n} checks:\n\n- the **subtotal** of every line, before the coupon\n- the coupon, which takes off a share or a fixed sum\n  - a share rounds once, on the subtotal\n  - a fixed sum never takes the total below zero\n- the tax, from the address the owner typed\n- the shipping, which is free over \`50.00\``,
    `\`\`\`ts\nexport function total(lines: Line[], coupon?: Coupon): number {\n  const subtotal = lines.reduce((sum, line) => sum + line.price * line.quantity, 0)\n  const off = coupon ? discount(subtotal, coupon) : 0\n  const taxed = (subtotal - off) * (1 + taxRate(lines))\n  return Math.round(taxed * 100) / 100\n}\n\nfunction discount(subtotal: number, coupon: Coupon): number {\n  if (coupon.kind === 'share') return Math.round(subtotal * coupon.share * 100) / 100\n  return Math.min(subtotal, coupon.amount)\n}\n\`\`\``,
    `| Case | Cart | Order | Step ${n} |\n| --- | ---: | ---: | --- |\n| two lines of 0.005 | 0.02 | 0.01 | shows 0.01 |\n| a 10% coupon on 9.99 | 8.99 | 8.99 | passes |\n| a 5.00 coupon on 3.00 | 0.00 | 0.00 | passes |\n| free shipping at 50.00 | 50.00 | 50.00 | passes |`,
    `1. Read the cart from the store.\n2. Ask the server for the order.\n3. Compare them, and keep the order's figure when they differ.`,
  ].join('\n\n')
}).join('\n\n')

// backgroundAgents are the two subagents a message that says "agents" starts: what each looks into, what it is asked and
// on which model, '' for the thread's own, the tools it runs, and the report it ends with.
const backgroundAgents = [
  {
    description: 'List the checkout pages',
    type: 'Explore',
    prompt: 'List every page of the checkout under app-frontend/src/js/pages. Give each file name, nothing else.',
    model: 'haiku',
    steps: [{ tool: 'Glob', input: { pattern: 'app-frontend/src/js/pages/*.tsx' }, result: 'app-frontend/src/js/pages/Cart.tsx\napp-frontend/src/js/pages/Checkout.tsx\napp-frontend/src/js/pages/checkout_steps.tsx' }],
    report: 'Three pages: `Cart.tsx`, `Checkout.tsx` and `checkout_steps.tsx`.',
  },
  {
    description: 'Count the checkout tests',
    type: 'general-purpose',
    prompt: 'Count the tests in app-frontend/test that cover the checkout. Say which file holds them.',
    model: '',
    steps: [
      { tool: 'Grep', input: { pattern: 'test\\(', path: 'app-frontend/test' }, result: 'app-frontend/test/checkout.test.ts' },
      { tool: 'Bash', input: { command: "grep -c 'test(' app-frontend/test/checkout.test.ts", description: 'Count the tests' }, result: '12' },
    ],
    report: '`checkout.test.ts` holds 12 tests.',
  },
]
const agentStepEvery = 1500

const longMessage = Array.from({ length: 14 }, (_, i) => `${i + 1}. Check that step ${i + 1} of the checkout keeps the cart total, the coupon and the address the owner typed.`).join('\n')

// olderTurns is finished turns that give a chat at least that many items: each a message and its reply, every fifth one
// a tool call that folds into a work item, every ninth one an error that ends it, every eleventh a long message and
// every thirteenth a second message sent right after the first.
function olderTurns(items: number, ago: (minutes: number) => number): Draft[] {
  const turns: Draft[] = []
  for (let n = 1, count = 0; count < items; n++) {
    const at = ago(38 + items - count)
    const text = n % 11 === 0 ? `Question ${n}: go through the checkout steps:\n${longMessage}` : n % 7 === 0 ? `Question ${n}: what does the cart do with a coupon\nthat has run out?` : `Question ${n}: check the cart again.`
    turns.push({ kind: 'user', id: `old-u-${n}`, text, at })
    if (n % 13 === 0) {
      turns.push({ kind: 'user', id: `old-u-${n}b`, text: 'And the coupon strip too.', at })
      count++
    }
    if (n % 5 === 0) {
      turns.push({ kind: 'tool_use', id: `old-t-${n}`, tool: 'Read', input: { file_path: 'app-frontend/src/js/cart.ts' }, at })
      turns.push({ kind: 'tool_result', parent: `old-t-${n}`, text: 'export const cart = {}', at })
      count++
    }
    turns.push({ kind: 'message', id: `old-m-${n}`, uuid: `old-r-${n}`, text: olderReplies[n % olderReplies.length], at })
    if (n % 9 === 0) {
      turns.push({ kind: 'turn_end', error: true, text: 'API Error: 529 {"type":"overloaded_error","message":"Overloaded"}', cost: 0.01, at })
      count++
    } else turns.push({ kind: 'turn_end', cost: 0.01, at })
    count += 2
  }
  return turns
}

// mockChat stands in for the chat service: logs kept in memory, chat:<id> events for watched threads in batches, and a
// reply streamed as deltas over a second, or over eight to a message that says "slow", which leaves time to queue
// another, after three tool calls of a second and a half each to a message that says "tools", or about 30 KB of
// markdown, as Claude streams it, to a message that says "long", or two subagents in the background to a message that
// says "agents", whose Bash call waits on an approval when the message also says "ask". Claude exits on a message that
// is just "exit". The images sent are kept in memory, at the paths the service would save them to, and mediaURL gives
// each one's address. thread throws for an id that has no chat, and isProject says whether a project key is configured.
export function mockChat(emit: Emit, thread: (id: string) => MockThread, isProject: (key: string) => boolean, slow = 0) {
  const logs = new Map<string, ChatEvent[]>()
  const unreadable = new Set<string>()
  const watched = new Set<string>()
  const pending = new Map<string, Pending>()
  const queued = new Map<string, { id: string; text: string; files: string[] }[]>()
  const media = new Map<string, string>() // a saved image's data URL, by its path
  const replying = new Map<string, () => void>() // stops the reply each thread is streaming
  // The subagents each thread runs in the background, by call, how to stop them, and how to go on once the approval one
  // of them waits on is answered.
  const background = new Map<string, { stop(): void; calls: string[]; resume?: (allow: boolean) => void }>()
  const batches = new Map<string, { events: ChatEvent[]; due: boolean; sent: number }>()
  // The command list a thread of each project reported last, as the service keeps it for Home.
  const projectCommands = new Map<string, { at: number; list: typeof commands }>()
  let nextReply = 0

  const logOf = (id: string) => logs.get(id) ?? []

  function append(id: string, draft: Draft): ChatEvent {
    const log = logOf(id)
    const at = draft.at ?? Date.now()
    const e: ChatEvent = { ...draft, at, seq: draft.kind === 'delta' || draft.kind === 'thinking' ? 0 : (log.at(-1)?.seq ?? 0) + 1 }
    if (e.seq > 0) logs.set(id, [...log, e])
    if (e.kind === 'commands') keepCommands(id.slice(0, id.indexOf('/')), e)
    if (watched.has(id)) batch(id, e)
    return e
  }

  function known(project: string) {
    if (!isProject(project)) throw `No project ${project}`
  }

  function keepCommands(project: string, e: ChatEvent) {
    if ((projectCommands.get(project)?.at ?? 0) >= e.at) return
    projectCommands.set(project, { at: e.at, list: e.input as typeof commands })
  }

  function batch(id: string, e: ChatEvent) {
    const b = batches.get(id) ?? { events: [], due: false, sent: 0 }
    batches.set(id, b)
    b.events.push({ ...e })
    if (b.due) return
    b.due = true
    setTimeout(() => {
      const events = b.events
      Object.assign(b, { events: [], due: false, sent: Date.now() })
      if (watched.has(id)) emit(`chat:${id}` as `chat:${string}/${string}`, { id, events })
    }, Math.max(0, batchEvery - (Date.now() - b.sent)))
  }

  function setPending(id: string, p: Pending | null) {
    if (p) pending.set(id, p)
    else if (!pending.delete(id)) return
    emit('chat:pending', { id, pending: p })
  }

  function live(id: string): MockThread {
    const t = thread(id)
    if (t.settled) throw `Thread ${id} is asleep`
    return t
  }

  // awake is the thread, woken first when it has settled, as a message or a draft to a settled thread wakes it.
  function awake(id: string): MockThread {
    const t = thread(id)
    if (t.settled) t.wake()
    return t
  }

  // reply answers the message asked, with three tool calls first when it says "tools", as stream does.
  function reply(id: string, asked = '') {
    const calls = /\btools\b/i.test(asked) ? toolCalls : []
    const call = (k: number) => {
      if (k === calls.length) {
        stream(id, asked)
        return
      }
      const use = `toolu_${Math.random().toString(36).slice(2, 10)}`
      append(id, { kind: 'tool_use', id: use, tool: calls[k].tool, input: calls[k].input })
      const timer = setTimeout(() => {
        append(id, { kind: 'tool_result', parent: use, text: calls[k].result })
        call(k + 1)
      }, toolRuns)
      replying.set(id, () => clearTimeout(timer))
    }
    call(0)
  }

  // stream streams Claude's thinking, then its reply to the message asked, at the pace of a reply over a second, or over
  // eight to a message that says "slow", then ends the turn, or runs the next queued message as its own turn.
  function stream(id: string, asked: string) {
    const t = thread(id)
    const n = nextReply++
    const long = /\blong\b/i.test(asked)
    const text = long ? longReply : replies[n % replies.length]
    const thought = thoughts[n % thoughts.length]
    const words = long ? text.match(/[^]{1,15}/g)! : text.split(/(?<= )/)
    const thinking = thought.split(/(?<= )/)
    const message = `msg_${Math.random().toString(36).slice(2, 10)}`
    let i = 0
    let j = -1
    const timer = setInterval(() => {
      if (j < thinking.length) {
        if (j === -1) append(id, { kind: 'thinking', id: message })
        else append(id, { kind: 'thinking', id: message, text: thinking[j] })
        if (++j === thinking.length) append(id, { kind: 'thought', id: message, text: thought })
        return
      }
      if (i < words.length) {
        append(id, { kind: 'delta', id: message, text: words[i++] })
        return
      }
      clearInterval(timer)
      replying.delete(id)
      append(id, { kind: 'message', id: message, uuid: crypto.randomUUID(), text })
      if (runQueued(id)) return
      append(id, { kind: 'turn_end', cost: 0.04 })
      t.setState('idle')
    }, long ? longDeltaEvery : (/\bslow\b/i.test(asked) ? 8000 : 1000) / words.length)
    replying.set(id, () => clearInterval(timer))
  }

  // subagents runs a turn that starts two subagents in the background, as Claude does by default: the turn ends once
  // both are launched, each then works and ends on its own, and Claude reports both in a turn of its own, with no
  // message of the owner's. The row works until that turn ends, as the threads service shows it. With ask, the subagents' Bash
  // call waits on the owner's approval.
  function subagents(id: string, ask: boolean) {
    const t = thread(id)
    const runs = backgroundAgents.map((a) => ({ ...a, call: `toolu_${Math.random().toString(36).slice(2, 10)}`, done: 0 }))
    t.setState('working')
    for (const r of runs) {
      append(id, { kind: 'tool_use', id: r.call, tool: 'Agent', input: { description: r.description, subagent_type: r.type, prompt: r.prompt, ...(r.model ? { model: r.model } : {}) } })
      // Claude lists its tasks again just before each start, and after each end.
      append(id, { kind: 'tasks', input: runs.slice(0, runs.indexOf(r) + 1).map((x) => ({ id: `task-${x.call}`, kind: 'local_agent' })) })
      append(id, { kind: 'task_start', id: `task-${r.call}`, parent: r.call, tool: 'local_agent', text: r.description })
    }
    for (const r of runs) append(id, { kind: 'tool_result', parent: r.call, text: 'Async agent launched successfully.' })
    append(id, { kind: 'message', id: `msg_${Math.random().toString(36).slice(2, 10)}`, uuid: crypto.randomUUID(), text: 'Both subagents are running. I will report what they find.' })
    append(id, { kind: 'turn_end', cost: 0.02 })
    const open = runs.map((r) => r.call)
    t.setSubagents(open.length)
    let waiting = false
    const timer = setInterval(() => {
      if (waiting) return
      const r = runs.find((r) => r.done <= r.steps.length)
      if (!r) {
        clearInterval(timer)
        background.delete(id)
        append(id, { kind: 'message', id: `msg_${Math.random().toString(36).slice(2, 10)}`, uuid: crypto.randomUUID(), text: runs.map((r) => r.report).join(' ') })
        append(id, { kind: 'turn_end', cost: 0.01 })
        t.setState('idle')
        return
      }
      const step = r.steps[r.done]
      const tool = `toolu_${Math.random().toString(36).slice(2, 10)}`
      if (step && ask && step.tool === 'Bash') {
        append(id, { kind: 'tool_use', id: tool, tool: step.tool, input: step.input, agent: r.call })
        append(id, { kind: 'approval', id: `req-${tool}`, parent: tool, tool: step.tool, input: step.input, agent: r.call })
        setPending(id, { id, request: `req-${tool}`, tool: step.tool, summary: (step.input as { command: string }).command, kind: 'tool', input: step.input, inputTooLarge: false })
        waiting = true
        const run = background.get(id)
        if (run)
          run.resume = (allow) => {
            append(id, { kind: 'tool_result', parent: tool, text: allow ? step.result : 'The user denied this tool call.', error: !allow, agent: r.call })
            waiting = false
          }
      } else if (step) {
        append(id, { kind: 'tool_use', id: tool, tool: step.tool, input: step.input, agent: r.call })
        append(id, { kind: 'tool_result', parent: tool, text: step.result, agent: r.call })
      } else {
        append(id, { kind: 'message', id: `msg_${Math.random().toString(36).slice(2, 10)}`, text: r.report, agent: r.call })
        append(id, { kind: 'task_end', parent: r.call, text: r.report })
        open.splice(open.indexOf(r.call), 1)
        append(id, { kind: 'tasks', input: open.map((call) => ({ id: `task-${call}`, kind: 'local_agent' })) })
        t.setSubagents(open.length)
      }
      r.done++
      // The subagents work side by side, so the next step is the other one's.
      runs.push(...runs.splice(runs.indexOf(r), 1))
    }, agentStepEvery)
    background.set(id, { stop: () => clearInterval(timer), calls: open })
  }

  // runQueued takes in every queued message and replies to them, as Claude does once it can.
  function runQueued(id: string): boolean {
    const held = queued.get(id) ?? []
    if (held.length === 0) return false
    queued.delete(id)
    held.forEach((m) => append(id, { kind: 'user', id: m.id, text: m.text, files: m.files }))
    reply(id)
    return true
  }

  // stop ends the turn that runs, as an interrupt does. Claude then runs the messages it still holds, in order; like the
  // fake claude of the Go tests, it no longer holds one whose text is "lost", and the host logs that one unsent.
  function stop(id: string) {
    const running = replying.get(id)
    const waiting = pending.get(id)
    const asked = waiting && logOf(id).find((e) => e.kind === 'approval' && e.id === waiting.request)
    // A subagent's approval outlives the interrupt, as its subagent does.
    const own = waiting !== undefined && !asked?.agent
    if (!running && !own) return
    running?.()
    replying.delete(id)
    // Claude withdraws the approval that waits and rejects its call.
    if (own) {
      append(id, { kind: 'cancel', id: waiting.request })
      append(id, { kind: 'tool_result', parent: asked?.parent, text: "The user doesn't want to proceed with this tool use.", error: true })
      setPending(id, null)
    }
    const held = queued.get(id) ?? []
    held.filter((m) => m.text === 'lost').forEach((m) => append(id, { kind: 'unsent', id: m.id, text: m.text }))
    queued.set(id, held.filter((m) => m.text !== 'lost'))
    append(id, { kind: 'turn_end', error: true, cost: 0.04 })
    if (runQueued(id)) return
    thread(id).setState('idle')
  }

  // seed fills a thread's log with two turns, the second of which waits on a Bash approval, after earlier turns that
  // give the chat about that many more items.
  function seed(id: string, earlier = 0) {
    const ago = (minutes: number) => Date.now() - minutes * minute
    const turns: Draft[] = [
      { kind: 'start', at: ago(38 + earlier) },
      { kind: 'commands', input: commands, at: ago(38 + earlier) },
      ...olderTurns(earlier, ago),
      { kind: 'user', id: 'u-1', text: 'Move checkout to its own route, and keep the cart page as it is.', at: ago(38) },
      { kind: 'init', id: 'mock-session', at: ago(38) },
      { kind: 'thought', id: 'msg_1', text: '**Finding the routes**\n\nThe router is the one place that lists the pages, so I read it before I add a route.', at: ago(37) },
      { kind: 'message', id: 'msg_1', uuid: 'r-1', text: "I'll read the router first.", at: ago(37) },
      { kind: 'tool_use', id: 'toolu_1', tool: 'Read', input: { file_path: '~/code/acme/storefront/trees/issue-444/app-frontend/src/js/routes.tsx' }, at: ago(37) },
      { kind: 'tool_result', parent: 'toolu_1', text: "export const routes = [\n  { path: '/', page: Home },\n  { path: '/cart', page: Cart },\n]", at: ago(37) },
      { kind: 'tool_use', id: 'toolu_2', tool: 'Edit', input: { file_path: '~/code/acme/storefront/trees/issue-444/app-frontend/src/js/routes.tsx', old_string: "  { path: '/cart', page: Cart },", new_string: "  { path: '/cart', page: Cart },\n  { path: '/checkout', page: Checkout }," }, at: ago(36) },
      { kind: 'tool_result', parent: 'toolu_2', text: 'The file was updated.', at: ago(36) },
      { kind: 'message', id: 'msg_1b', uuid: 'r-1b', text: 'The route is in. Now I will **lint** and typecheck it.', at: ago(36) },
      { kind: 'tool_use', id: 'toolu_2a', tool: 'Bash', input: { command: 'npm run lint\nnpm run typecheck', description: 'Check the new route' }, at: ago(36) },
      { kind: 'tool_result', parent: 'toolu_2a', text: Array.from({ length: 26 }, (_, i) => `src/js/file-${i + 1}.tsx  ok`).join('\n'), at: ago(36) },
      { kind: 'tool_use', id: 'toolu_2b', tool: 'Bash', input: { command: 'cat src/js/missing.tsx' }, at: ago(36) },
      { kind: 'tool_result', parent: 'toolu_2b', text: 'cat: src/js/missing.tsx: No such file or directory', error: true, at: ago(36) },
      { kind: 'tool_use', id: 'toolu_2c', tool: 'Grep', input: { pattern: 'Checkout' }, at: ago(36) },
      { kind: 'tool_result', parent: 'toolu_2c', text: 'app-frontend/src/js/routes.tsx', at: ago(36) },
      { kind: 'message', id: 'msg_2', uuid: 'r-2', text: 'Checkout now has its own route at `/checkout`. The cart page is unchanged. Its steps are set in [docs/config.md](docs/config.md).', at: ago(35) },
      { kind: 'turn_end', cost: 0.21, at: ago(35) },
      { kind: 'user', id: 'u-2', text: 'Run the tests, and install whatever they need.', at: ago(5) },
      { kind: 'init', id: 'mock-session', at: ago(5) },
      { kind: 'thought', id: 'msg_3', text: 'The test run failed on an import of a package that is not in node_modules. Installing it should fix the run.', at: ago(4) },
      { kind: 'message', id: 'msg_3', uuid: 'r-3', text: 'The tests need a package that is not installed yet.', at: ago(4) },
      { kind: 'tool_use', id: 'toolu_3', tool: 'Bash', input: { command: 'npm install', description: 'Install the missing packages' }, at: ago(4) },
      { kind: 'approval', id: 'req-1', parent: 'toolu_3', tool: 'Bash', input: { command: 'npm install', description: 'Install the missing packages' }, at: ago(4) },
    ]
    turns.forEach((e) => append(id, e))
    setPending(id, { id, request: 'req-1', tool: 'Bash', summary: 'npm install', kind: 'tool', input: { command: 'npm install', description: 'Install the missing packages' }, inputTooLarge: false })
  }

  // answer logs the answer and the tool's result, with input when it allows with the user's own, then replies.
  function answer(id: string, request: string, allow: boolean, message: string, input?: { answers?: Record<string, string> }) {
    const t = live(id)
    const p = pending.get(id)
    if (p?.request !== request) throw `No approval "${request}" is waiting`
    append(id, { kind: 'answer', id: request, text: allow ? 'allow' : 'deny', ...(input ? { input } : {}) })
    setPending(id, null)
    const asked = logOf(id).find((e) => e.kind === 'approval' && e.id === request)
    // A subagent's approval lets its subagent go on, beside the turns.
    if (asked?.agent) {
      background.get(id)?.resume?.(allow)
      return
    }
    const answered = Object.entries(input?.answers ?? {}).map(([q, a]) => `"${q}"="${a}"`)
    const allowed = { tool: 'added 12 packages in 3s', question: `The user answered: ${answered.join(', ')}.`, plan: 'User has approved your plan. You can now start coding.' }
    append(id, { kind: 'tool_result', parent: asked?.parent, text: allow ? allowed[p.kind] : message.trim() || 'The user denied this tool call.', error: !allow })
    // Claude leaves plan mode once its plan is approved, and reports it.
    if (allow && p.kind === 'plan') {
      mode(id, 'default')
      t.followMode('default')
    }
    t.setState('working')
    reply(id)
  }

  // ask fills a chat's log with one turn that waits on Claude's question or plan.
  function ask(id: string, kind: 'question' | 'plan') {
    const at = Date.now() - 2 * minute
    const tool = kind === 'question' ? 'AskUserQuestion' : 'ExitPlanMode'
    const input = kind === 'question' ? { questions: askedQuestions } : { plan: askedPlan }
    const turn: Draft[] = [
      { kind: 'start', at },
      { kind: 'commands', input: commands, at },
      { kind: 'user', id: 'u-1', text: kind === 'question' ? 'Why does checkout call the cart API twice?' : 'Plan how to split the checkout page.', at },
      { kind: 'init', id: 'mock-session', at },
      { kind: 'message', id: 'msg_1', uuid: 'r-ask', text: kind === 'question' ? 'Checkout mounts the old cart hook and the new one, and each fetches the cart.' : 'I read the checkout page and its tests.', at },
      { kind: 'tool_use', id: 'toolu_ask', tool, input, at },
      { kind: 'approval', id: 'req-ask', parent: 'toolu_ask', tool, input, at },
    ]
    turn.forEach((e) => append(id, e))
    const summary = kind === 'question' ? askedQuestions[0].question : 'Split checkout into three steps'
    setPending(id, { id, request: 'req-ask', tool, summary, kind, input, inputTooLarge: false })
  }

  // delegations fills a chat's log with a turn that delegates a task to each child, and one call that fails, then, for
  // each child with a result, the result as the child's message and Claude's reply to it, and a message from another
  // thread, as the orchestrator tools log them.
  function delegations(id: string, children: MockChild[], sender: { id: string; title: string }) {
    const ago = (minutes: number) => Date.now() - minutes * minute
    const app = 'APPLICATION_PROMPT: '
    const reply = (n: string, text: string, at: number): Draft[] => [
      { kind: 'init', id: 'mock-session', at },
      { kind: 'message', id: `msg_${n}`, uuid: `r-${n}`, text, at: at + 14_000 },
      { kind: 'turn_end', cost: 0.03, at: at + 14_000 },
    ]
    const turns: Draft[] = [
      { kind: 'start', at: ago(20) },
      { kind: 'commands', input: commands, at: ago(20) },
      { kind: 'user', id: 'u-1', text: 'The checkout tests fail now and then. Find the flaky one, and check whether the notes sync has the same race.', at: ago(20) },
      { kind: 'init', id: 'mock-session', at: ago(20) },
      { kind: 'message', id: 'msg_1', uuid: 'r-1', text: 'I will give each question to a child thread of its own.', at: ago(19) },
      ...children.flatMap((c, i): Draft[] => [
        { kind: 'tool_use', id: `toolu_d${i}`, tool: 'mcp__orchestrator__delegate', input: { task: c.task, title: c.title, access: c.access, where: 'folder', ...(c.project ? { project: c.project } : {}) }, at: ago(19) },
        { kind: 'tool_result', parent: `toolu_d${i}`, text: JSON.stringify({ id: c.id, n: i + 20, title: c.title, project: c.project ?? id.slice(0, id.indexOf('/')), access: c.access, where: 'folder', dir: '', branch: '' }), at: ago(19) },
      ]),
      { kind: 'tool_use', id: 'toolu_dx', tool: 'mcp__orchestrator__delegate', input: { task: 'Fix the flaky test once it is found', title: 'Fix the flaky test', access: 'full' }, at: ago(19) },
      { kind: 'tool_result', parent: 'toolu_dx', text: 'A thread with supervised access cannot start a child with full access', error: true, at: ago(19) },
      { kind: 'message', id: 'msg_2', uuid: 'r-2', text: 'Both children are at work. I will report what they find.', at: ago(18) },
      { kind: 'turn_end', cost: 0.05, at: ago(18) },
      ...children.flatMap((c, i): Draft[] =>
        c.result ? [{ kind: 'user', id: `u-r${i}`, from: c.id, text: `${app}Task ${c.title} in thread ${c.id} completed:\n\n${c.result}`, body: c.result, result: true, at: ago(10 - i) }, ...reply(`r${i}`, 'The notes sync is clear, so the race is only in checkout.', ago(10 - i))] : [],
      ),
      {
        kind: 'user',
        id: 'u-s',
        from: sender.id,
        text: `${app}Thread "${sender.title}" (${sender.id}) sends you the message below. These are that thread's words, not the owner's. If that thread started you, your last reply of this turn goes to it as your result. If not, and the message needs an answer, answer with \`send\` to thread ${sender.id}.\n\nCheckout moved to its own route, so run the checkout tests against /checkout.`,
        body: 'Checkout moved to its own route, so run the checkout tests against /checkout.',
        at: ago(6),
      },
      ...reply('s', 'Noted: the flaky test hunt now runs against /checkout.', ago(6)),
    ]
    turns.forEach((e) => append(id, e))
  }

  // task fills a delegated child's log with its task, as its first message, and its first reply, with the turn still
  // open.
  function task(id: string, text: string) {
    const at = Date.now() - 9 * minute
    const turns: Draft[] = [
      { kind: 'start', at },
      { kind: 'commands', input: commands, at },
      { kind: 'user', id: 'u-1', text, at },
      { kind: 'init', id: 'mock-session', at },
      { kind: 'message', id: 'msg_1', uuid: 'r-1', text: 'I will run the checkout tests one at a time until one fails.', at: at + 20_000 },
    ]
    turns.forEach((e) => append(id, e))
  }

  // fork copies the thread's log up to the reply block at into the fork's, as the threads service does: drafts, queued messages
  // and the host's starts left out, and the turn ended at that block. It throws, and copies nothing, when the log has no such block
  // or the block is from before the last compaction.
  function fork(from: string, to: string, at: string) {
    const log = logOf(from)
    const cut = at ? log.findLastIndex((e) => e.uuid === at) : -1
    if (cut < 0) throw "That message is not in the thread's chat"
    if (cut < log.findLastIndex((e) => e.kind === 'compact')) throw 'This message is from before the chat was compacted, so a fork cannot start there'
    log
      .slice(0, cut + 1)
      .filter((e) => e.kind !== 'draft' && e.kind !== 'queued' && e.kind !== 'unsent' && e.kind !== 'start')
      .forEach(({ seq: _, ...e }) => append(to, e))
    append(to, { kind: 'turn_end', at: log[cut].at })
  }

  // latestReply is the uuid of the last block of the thread's latest reply that a turn's end follows, where a side chat
  // forks it, as the threads service finds it: a rewind takes its message back, and it throws when there is none or it is from
  // before the last compaction.
  function latestReply(id: string): string {
    const log = logOf(id)
    let ended = false
    for (let i = log.length - 1; i >= 0; i--) {
      const e = log[i]
      if (e.kind === 'compact') {
        if (log.slice(0, i).some((m) => m.kind === 'message' && m.uuid)) throw "This thread's last reply is from before its chat was compacted. Side chats start after its next reply."
        break
      }
      if (e.kind === 'rewind') {
        const from = log.slice(0, i).findIndex((u) => u.kind === 'user' && u.id === e.id)
        if (from >= 0) i = from
      } else if (e.kind === 'turn_end') ended = true
      else if (ended && e.kind === 'message' && e.uuid && !e.agent) return e.uuid
    }
    throw 'Side chats start after the first message.'
  }

  // sideChatAsked says whether the side chat was sent a message of its own, after its host first started.
  function sideChatAsked(id: string): boolean {
    const log = logOf(id)
    const started = log.findIndex((e) => e.kind === 'start')
    return started >= 0 && log.slice(started).some((e) => e.kind === 'user' && !e.parent)
  }

  // sideChatBlock is the side chat's questions and answers as a message that carries it gives them to Claude, in the
  // words of internal/prompts/side-chat.md.
  function sideChatBlock(title: string, id: string): string {
    const log = logOf(id)
    const started = log.findIndex((e) => e.kind === 'start')
    const turns: { question: string; answer: string[] }[] = []
    for (const e of started < 0 ? [] : log.slice(started)) {
      if (e.kind === 'user' && !e.parent) turns.push({ question: e.body ?? e.text ?? '', answer: [] })
      else if (e.kind === 'message' && !e.agent && e.text?.trim()) turns.at(-1)?.answer.push(e.text)
    }
    const exchange = turns.map((t) => `Question:\n${t.question}\n\nAnswer:\n${t.answer.join('\n\n') || '(no answer yet)'}`).join('\n\n')
    const quoted = title.trim().split(/\s+/).join(' ').replaceAll('"', "'")
    return `The owner attaches the side chat "${quoted}" to this message. A side chat is a fork of this thread where the owner asked questions on the side; below are its questions and answers, oldest first. They are context for this message, not a task of their own.\n\n${exchange}`
  }

  // forget stops the turn that waits, as an interrupt from elsewhere does, but the app hears of neither the pending
  // change nor the thread's state, as when that event was missed: the row still offers Allow, and the chat's card
  // expires.
  function forget(id: string) {
    const request = pending.get(id)?.request
    pending.delete(id)
    append(id, { kind: 'interrupt' })
    if (request) append(id, { kind: 'cancel', id: request })
    append(id, { kind: 'turn_end', error: true, cost: 0.02 })
  }

  const backend = {
    ChatHistory: async (id: string, after: number) => {
      thread(id)
      if (slow) await new Promise((resolve) => setTimeout(resolve, slow))
      if (unreadable.has(id)) throw `The chat log of ${id} could not be read`
      return logOf(id).filter((e) => e.seq > after).map((e) => ({ ...e }))
    },
    ChatWatch: async (id: string) => {
      live(id)
      watched.add(id)
    },
    ChatUnwatch: async (id: string) => {
      watched.delete(id)
    },
    // body is the bare words of a text that frames them for Claude, which the chat shows.
    ChatSend: async (id: string, text: string, images: ChatImage[], body?: string) => {
      const t = awake(id)
      const files = save(id, images)
      const message = crypto.randomUUID()
      const waiting = pending.get(id)
      if (waiting && files.length > 0) throw 'Answer the approval before sending images'
      if (waiting) {
        const asked = logOf(id).find((e) => e.kind === 'approval' && e.id === waiting.request)
        append(id, { kind: 'answer', id: waiting.request, text: 'deny' })
        append(id, { kind: 'user', id: message, text, parent: waiting.request, ...(asked?.agent ? { agent: asked.agent } : {}) })
        setPending(id, null)
        // The text tells the subagent that asked why, and starts no turn.
        if (asked?.agent) {
          background.get(id)?.resume?.(false)
          return { id: message }
        }
        append(id, { kind: 'tool_result', parent: asked?.parent, text, error: true })
        t.setState('working')
        reply(id, text)
        return { id: message }
      }
      if (replying.has(id)) {
        append(id, { kind: 'queued', id: message, text, files })
        queued.set(id, [...(queued.get(id) ?? []), { id: message, text, files }])
        return { id: message }
      }
      append(id, { kind: 'user', id: message, text, files, ...(body !== undefined ? { body } : {}) })
      if (text === 'exit') {
        end(id)
        t.setState('settled')
        return { id: message }
      }
      if (text === '/compact') {
        append(id, { kind: 'compact' })
        append(id, { kind: 'init', id: 'mock-session' })
        append(id, { kind: 'turn_end', cost: 0.01 })
        return { id: message }
      }
      append(id, { kind: 'init', id: 'mock-session' })
      if (/\bagents\b/i.test(text)) {
        subagents(id, /\bask\b/i.test(text))
        return { id: message }
      }
      t.setState('working')
      reply(id, text)
      return { id: message }
    },
    ChatDraft: async (id: string, text: string) => {
      awake(id)
      append(id, { kind: 'draft', text })
    },
    ChatInterrupt: async (id: string) => {
      live(id)
      append(id, { kind: 'interrupt' })
      stop(id)
    },
    ChatSendNow: async (id: string, message: string) => {
      live(id)
      if (!queued.get(id)?.some((m) => m.id === message)) throw `No queued message ${message}`
      append(id, { kind: 'interrupt', id: message })
      stop(id)
    },
    ChatAnswer: async (id: string, request: string, allow: boolean, message: string) => answer(id, request, allow, message),
    ChatAnswerInput: async (id: string, request: string, input: string) => {
      let given
      try {
        given = JSON.parse(input)
      } catch {
        throw 'The answer is not valid JSON'
      }
      answer(id, request, true, '', given)
    },
    ChatPending: async () => [...pending.values()].map((p) => ({ ...p })),
    ChatFiles: async (id: string, query: string) => {
      thread(id)
      return rankFiles(files, query)
    },
    RewindFilesOffer: async (id: string): Promise<FilesRewind> => {
      const t = thread(id)
      return { offered: t.worktree !== '', reason: t.worktree ? t.filesReason() : '', files: [], insertions: 0, deletions: 0 }
    },
    RewindPreview: async (id: string, message: string): Promise<FilesRewind> => {
      const t = thread(id)
      if (!t.worktree) return { offered: false, reason: '', files: [], insertions: 0, deletions: 0 }
      if (t.filesReason()) return { offered: true, reason: t.filesReason(), files: [], insertions: 0, deletions: 0 }
      rewindable(id, message)
      return { offered: true, reason: '', ...editedSince(id, message) }
    },
    RewindTo: async (id: string, message: string, files: boolean) => {
      if (files && !thread(id).worktree) throw 'The files of the project folder are never rewound'
      if (files && thread(id).filesReason()) throw thread(id).filesReason()
      awake(id)
      rewindable(id, message)
      append(id, { kind: 'rewind', id: message })
      return logOf(id).find((e) => e.kind === 'user' && e.id === message)?.text ?? ''
    },
    RewindOff: async () => '',
    ProjectFiles: async (project: string, query: string) => {
      known(project)
      return rankFiles(files, query)
    },
    ChatReadFile: async (id: string, path: string) => {
      const t = thread(id)
      return readFile(path, t.worktree, `thread ${t.n}`)
    },
    ProjectReadFile: async (project: string, path: string) => {
      known(project)
      return readFile(path, '', `project ${project}`)
    },
    ChatFileTree: async (id: string) => {
      thread(id)
      return fileTree()
    },
    ProjectFileTree: async (project: string) => {
      known(project)
      return fileTree()
    },
    ChatFileDir: async (id: string, dir: string) => {
      thread(id)
      return listing(dir)
    },
    ProjectFileDir: async (project: string, dir: string) => {
      known(project)
      return listing(dir)
    },
    ProjectCommands: async (project: string) => {
      known(project)
      return (projectCommands.get(project)?.list ?? []).map((c) => ({ ...c }))
    },
  }

  // relative is path inside folder, the thread's worktree or '' for the project's, as the service resolves it.
  function relative(path: string, folder: string, of: string, what: string): string {
    const outside = `This ${what} is outside ${of}'s folder`
    if (path.startsWith('/') && (folder === '' || !path.startsWith(`${folder}/`))) throw outside
    const parts: string[] = []
    for (const step of (path.startsWith('/') ? path.slice(folder.length + 1) : path).split('/')) {
      if (step === '..' && parts.length === 0) throw outside
      if (step === '..') parts.pop()
      else if (step !== '' && step !== '.') parts.push(step)
    }
    return parts.join('/')
  }

  function fileTree(): FileListing {
    const entries: FileEntry[] = [
      ...files.map((path) => ({ path, dir: false, ignored: false })),
      ...ignoredRoot.map((path) => ({ path, dir: path in ignoredFolders, ignored: true })),
    ]
    return { entries: entries.sort((a, b) => (a.path < b.path ? -1 : 1)), truncated: false }
  }

  // listing is a folder's entries after the wait a folder takes to list.
  const listing = (dir: string) =>
    new Promise<FileListing>((resolve, reject) =>
      setTimeout(() => {
        try {
          resolve(fileDir(dir))
        } catch (err) {
          reject(err)
        }
      }, 400),
    )

  function fileDir(dir: string): FileListing {
    const rel = relative(dir, '', 'the folder', 'folder')
    const children = ignoredFolders[rel]
    if (children === undefined) throw 'This folder does not exist'
    return { entries: children.map((path) => ({ path, dir: path in ignoredFolders, ignored: true })), truncated: false }
  }

  // readFile is the file at path in folder as the service reads it: any regular file inside the folder.
  function readFile(path: string, folder: string, of: string): FileContent {
    const rel = relative(path, folder, of, 'file')
    const known = files.includes(rel) || ignoredRoot.includes(rel) || Object.values(ignoredFolders).some((children) => children.includes(rel))
    if (!known) throw 'This file does not exist'
    if (rel in ignoredFolders) throw 'This is not a file'
    const text = (content: string): FileContent => {
      const cut = content.length > maxText
      return { kind: 'text', text: cut ? content.slice(0, maxText) : content, size: content.length, truncated: cut, mime: '', data: '' }
    }
    if (rel.endsWith('.svg')) return { kind: 'image', text: '', size: logo.length, truncated: false, mime: 'image/svg+xml', data: btoa(logo) }
    if (rel.endsWith('.bin')) return { kind: 'binary', text: '', size: 48213, truncated: false, mime: '', data: '' }
    if (rel.endsWith('.log')) return text(Array.from({ length: 40_000 }, (_, i) => `2026-01-01T00:00:${String(i % 60).padStart(2, '0')}Z GET /cart 200 ${i}ms`).join('\n'))
    return text(markdown[rel] ?? source[rel] ?? `// ${rel}\n`)
  }

  // rewindable throws why the thread cannot rewind to before message now, as the host refuses it.
  function rewindable(id: string, message: string) {
    awake(id)
    if (busy(id)) throw 'Edit when the turn ends'
    const prompts = logOf(id).reduce<string[]>((ids, e) => {
      if (e.kind === 'user' && !e.parent && e.id) return [...ids, e.id]
      if (e.kind === 'rewind' && ids.includes(e.id ?? '')) return ids.slice(0, ids.indexOf(e.id ?? ''))
      return ids
    }, [])
    if (!prompts.includes(message)) throw "Claude's conversation no longer holds this message"
  }

  // editedSince is what Claude's edits since before message changed, as a dry run of a rewind of the files says.
  function editedSince(id: string, message: string): Pick<FilesRewind, 'files' | 'insertions' | 'deletions'> {
    const log = logOf(id)
    const lines = (s: unknown) => (typeof s === 'string' && s !== '' ? s.split('\n').length : 0)
    const changed = new Set<string>()
    let insertions = 0
    let deletions = 0
    for (const e of log.slice(log.findIndex((e) => e.kind === 'user' && e.id === message))) {
      const input = (e.input ?? {}) as { file_path?: string; old_string?: string; new_string?: string; content?: string }
      if (e.kind !== 'tool_use' || !['Edit', 'Write'].includes(e.tool ?? '') || !input.file_path) continue
      changed.add(input.file_path)
      insertions += lines(input.old_string)
      deletions += lines(input.new_string ?? input.content)
    }
    return { files: [...changed], insertions, deletions }
  }

  // save checks the images as the service does and keeps each at the path it would have, under the thread's
  // chat-media folder.
  function save(id: string, images: ChatImage[]): string[] {
    if (images.length > maxImages) throw `A message takes at most ${maxImages} images`
    images.forEach((image, i) => {
      if (image.base64.length > Math.ceil(maxImage / 3) * 4) throw `Image ${i + 1} is too large (the limit is ${maxImage / 2 ** 20} MB)`
      if (!(image.mime in imageExts)) throw `Image ${i + 1} is not png, jpeg, gif or webp`
    })
    const [key, token] = id.split('/')
    return images.map((image) => {
      const path = `/Users/mock/.local/share/orchestrator/${key}/chat-media/${token}/${Math.random().toString(16).slice(2, 18).padEnd(16, '0')}${imageExts[image.mime]}`
      media.set(path, `data:${image.mime};base64,${image.base64}`)
      return path
    })
  }

  const mediaURL = (path: string) => media.get(path)

  // end is the thread's process going: its reply stops, its queue goes and nothing waits any more. A settle or a
  // restart is the host stopping Claude itself, as its exit says; restart is the access the thread starts again with.
  function end(id: string, stop?: 'settle' | { restart: Access }) {
    replying.get(id)?.()
    replying.delete(id)
    const stopped = background.get(id)
    stopped?.stop()
    background.delete(id)
    queued.delete(id)
    setPending(id, null)
    if (!logs.has(id)) return
    // The host closes, as stopped, each task Claude still ran when it went.
    for (const call of stopped?.calls ?? []) append(id, { kind: 'task_end', parent: call, error: true, stop: 'stopped' })
    if (stopped) thread(id).setSubagents(0)
    if (stop === 'settle') append(id, { kind: 'exit', stop })
    else if (stop) append(id, { kind: 'exit', stop: 'restart', text: stop.restart })
    else append(id, { kind: 'exit' })
  }

  // start logs the run's start, as the host does before Claude runs, then the commands Claude lists a moment later.
  const start = (id: string) => {
    append(id, { kind: 'start' })
    setTimeout(() => append(id, { kind: 'commands', input: commands }), 1500)
  }

  // breakLog makes a thread's history fail to read, as a chat log the app cannot open does.
  const breakLog = (id: string) => unreadable.add(id)

  // busy says whether a turn runs, an approval waits, a message is queued or a subagent runs in the background, as the
  // host's hold op does; turnRuns leaves the subagents out.
  const turnRuns = (id: string) => replying.has(id) || pending.has(id) || (queued.get(id)?.length ?? 0) > 0
  const busy = (id: string) => turnRuns(id) || (background.get(id)?.calls.length ?? 0) > 0

  // mode logs Claude's new permission mode, as the host does once Claude changes it.
  const mode = (id: string, text: string) => append(id, { kind: 'mode', text })

  return { backend, seed, ask, delegations, task, fork, latestReply, sideChatAsked, sideChatBlock, forget, end, start, busy, turnRuns, mode, breakLog, mediaURL }
}
