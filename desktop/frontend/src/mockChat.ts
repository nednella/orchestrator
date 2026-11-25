import type { ChatEvent, ChatImage, EventMap, Pending, State } from './types'

type Emit = <K extends keyof EventMap>(event: K, payload: EventMap[K]) => void

// What the mock knows of a thread: whether it has ended, and a way to move its row's state as hooks would.
export type MockThread = { ended: boolean; setState(state: State): void }

type Draft = Omit<ChatEvent, 'seq' | 'at'> & { at?: number }

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
  'README.md',
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
]
const fileLimit = 30

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

// olderTurns is finished turns that give a chat at least that many items: each a message and its reply, and every
// fifth one a tool call that folds into a work item.
function olderTurns(items: number, ago: (minutes: number) => number): Draft[] {
  const turns: Draft[] = []
  for (let n = 1, count = 0; count < items; n++) {
    const at = ago(38 + items - count)
    turns.push({ kind: 'user', id: `old-u-${n}`, text: n % 7 === 0 ? `Question ${n}: what does the cart do with a coupon\nthat has run out?` : `Question ${n}: check the cart again.`, at })
    if (n % 5 === 0) {
      turns.push({ kind: 'tool_use', id: `old-t-${n}`, tool: 'Read', input: { file_path: 'app-frontend/src/js/cart.ts' }, at })
      turns.push({ kind: 'tool_result', parent: `old-t-${n}`, text: 'export const cart = {}', at })
      count++
    }
    turns.push({ kind: 'message', id: `old-m-${n}`, uuid: `old-r-${n}`, text: olderReplies[n % olderReplies.length], at })
    turns.push({ kind: 'turn_end', cost: 0.01, at })
    count += 2
  }
  return turns
}

// mockChat stands in for the chat service: logs kept in memory, chat:<id> events for watched threads in batches, and a
// reply streamed as deltas over a second, or over eight to a message that says "slow", which leaves time to queue
// another. Claude exits on a message that is just "exit". The images sent are kept in memory, at the paths the service
// would save them to, and mediaURL gives each one's address. thread throws for an id that has no chat.
export function mockChat(emit: Emit, thread: (id: string) => MockThread) {
  const logs = new Map<string, ChatEvent[]>()
  const unreadable = new Set<string>()
  const watched = new Set<string>()
  const pending = new Map<string, Pending>()
  const queued = new Map<string, { id: string; text: string; files: string[] }[]>()
  const media = new Map<string, string>() // a saved image's data URL, by its path
  const replying = new Map<string, () => void>() // stops the reply each thread is streaming
  const batches = new Map<string, { events: ChatEvent[]; due: boolean; sent: number }>()
  let nextReply = 0

  const logOf = (id: string) => logs.get(id) ?? []

  function append(id: string, draft: Draft): ChatEvent {
    const log = logOf(id)
    const at = draft.at ?? Date.now()
    const e: ChatEvent = { ...draft, at, seq: draft.kind === 'delta' ? 0 : (log.at(-1)?.seq ?? 0) + 1 }
    if (e.seq > 0) logs.set(id, [...log, e])
    if (watched.has(id)) batch(id, e)
    return e
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
    if (t.ended) throw `Thread ${id} has ended`
    return t
  }

  // reply streams a reply, then ends the turn, or runs the next queued message as its own turn.
  function reply(id: string, slow = false) {
    const t = thread(id)
    const text = replies[nextReply++ % replies.length]
    const words = text.split(/(?<= )/)
    const message = `msg_${Math.random().toString(36).slice(2, 10)}`
    let i = 0
    const timer = setInterval(() => {
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
    }, (slow ? 8000 : 1000) / words.length)
    replying.set(id, () => clearInterval(timer))
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
    if (!running && !pending.has(id)) return
    running?.()
    replying.delete(id)
    setPending(id, null)
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
      { kind: 'commands', input: commands, at: ago(38 + earlier) },
      ...olderTurns(earlier, ago),
      { kind: 'user', id: 'u-1', text: 'Move checkout to its own route, and keep the cart page as it is.', at: ago(38) },
      { kind: 'init', id: 'mock-session', at: ago(38) },
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
      { kind: 'message', id: 'msg_2', uuid: 'r-2', text: 'Checkout now has its own route at `/checkout`. The cart page is unchanged.', at: ago(35) },
      { kind: 'turn_end', cost: 0.21, at: ago(35) },
      { kind: 'user', id: 'u-2', text: 'Run the tests, and install whatever they need.', at: ago(5) },
      { kind: 'init', id: 'mock-session', at: ago(5) },
      { kind: 'message', id: 'msg_3', uuid: 'r-3', text: 'The tests need a package that is not installed yet.', at: ago(4) },
      { kind: 'tool_use', id: 'toolu_3', tool: 'Bash', input: { command: 'npm install', description: 'Install the missing packages' }, at: ago(4) },
      { kind: 'approval', id: 'req-1', parent: 'toolu_3', tool: 'Bash', input: { command: 'npm install', description: 'Install the missing packages' }, at: ago(4) },
    ]
    turns.forEach((e) => append(id, e))
    setPending(id, { id, request: 'req-1', tool: 'Bash', summary: 'npm install', kind: 'tool' })
  }

  // answer logs the answer and the tool's result, with input when it allows with the user's own, then replies.
  function answer(id: string, request: string, allow: boolean, message: string, input?: { answers?: Record<string, string> }) {
    const t = live(id)
    const p = pending.get(id)
    if (p?.request !== request) throw `No approval "${request}" is waiting`
    append(id, { kind: 'answer', id: request, text: allow ? 'allow' : 'deny', ...(input ? { input } : {}) })
    setPending(id, null)
    const asked = logOf(id).find((e) => e.kind === 'approval' && e.id === request)
    const answered = Object.entries(input?.answers ?? {}).map(([q, a]) => `"${q}"="${a}"`)
    const allowed = { tool: 'added 12 packages in 3s', question: `The user answered: ${answered.join(', ')}.`, plan: 'User has approved your plan. You can now start coding.' }
    append(id, { kind: 'tool_result', parent: asked?.parent, text: allow ? allowed[p.kind] : message.trim() || 'The user denied this tool call.', error: !allow })
    t.setState('working')
    reply(id)
  }

  // ask fills a chat's log with one turn that waits on Claude's question or plan.
  function ask(id: string, kind: 'question' | 'plan') {
    const at = Date.now() - 2 * minute
    const tool = kind === 'question' ? 'AskUserQuestion' : 'ExitPlanMode'
    const input = kind === 'question' ? { questions: askedQuestions } : { plan: askedPlan }
    const turn: Draft[] = [
      { kind: 'commands', input: commands, at },
      { kind: 'user', id: 'u-1', text: kind === 'question' ? 'Why does checkout call the cart API twice?' : 'Plan how to split the checkout page.', at },
      { kind: 'init', id: 'mock-session', at },
      { kind: 'message', id: 'msg_1', uuid: 'r-ask', text: kind === 'question' ? 'Checkout mounts the old cart hook and the new one, and each fetches the cart.' : 'I read the checkout page and its tests.', at },
      { kind: 'tool_use', id: 'toolu_ask', tool, input, at },
      { kind: 'approval', id: 'req-ask', parent: 'toolu_ask', tool, input, at },
    ]
    turn.forEach((e) => append(id, e))
    const summary = kind === 'question' ? askedQuestions[0].question : 'Split checkout into three steps'
    setPending(id, { id, request: 'req-ask', tool, summary, kind })
  }

  // fork copies the thread's log up to the reply block at into the fork's, as the sessions do: drafts and queued
  // messages left out, and the turn ended at that block. It throws, and copies nothing, when the log has no such block.
  function fork(from: string, to: string, at: string) {
    const log = logOf(from)
    const cut = at ? log.findLastIndex((e) => e.uuid === at) : -1
    if (cut < 0) throw "That message is not in the thread's chat"
    log
      .slice(0, cut + 1)
      .filter((e) => e.kind !== 'draft' && e.kind !== 'queued' && e.kind !== 'unsent')
      .forEach(({ seq: _, ...e }) => append(to, e))
    append(to, { kind: 'turn_end', at: log[cut].at })
  }

  // forget stops the turn that waits, as an interrupt from elsewhere does, but the app hears of neither the pending
  // change nor the thread's state, as when that event was missed: the row still offers Allow, and the chat's card
  // expires.
  function forget(id: string) {
    pending.delete(id)
    append(id, { kind: 'interrupt' })
    append(id, { kind: 'turn_end', error: true, cost: 0.02 })
  }

  const backend = {
    ChatHistory: async (id: string, after: number) => {
      thread(id)
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
    ChatSend: async (id: string, text: string, images: ChatImage[]) => {
      const t = live(id)
      const files = save(id, images)
      const message = crypto.randomUUID()
      const waiting = pending.get(id)
      if (waiting && files.length > 0) throw 'Answer the approval before sending images'
      if (waiting) {
        append(id, { kind: 'answer', id: waiting.request, text: 'deny' })
        append(id, { kind: 'user', id: message, text, parent: waiting.request })
        const asked = logOf(id).find((e) => e.kind === 'approval' && e.id === waiting.request)
        append(id, { kind: 'tool_result', parent: asked?.parent, text, error: true })
        setPending(id, null)
        t.setState('working')
        reply(id)
        return { id: message }
      }
      if (replying.has(id)) {
        append(id, { kind: 'queued', id: message, text, files })
        queued.set(id, [...(queued.get(id) ?? []), { id: message, text, files }])
        return { id: message }
      }
      append(id, { kind: 'user', id: message, text, files })
      if (text === 'exit') {
        end(id)
        t.setState('ended')
        return { id: message }
      }
      append(id, { kind: 'init', id: 'mock-session' })
      t.setState('working')
      reply(id, /\bslow\b/i.test(text))
      return { id: message }
    },
    ChatDraft: async (id: string, text: string) => {
      live(id)
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
      const path = `/Users/mock/.local/share/agentos/${key}/chat-media/${token}/${Math.random().toString(16).slice(2, 18).padEnd(16, '0')}${imageExts[image.mime]}`
      media.set(path, `data:${image.mime};base64,${image.base64}`)
      return path
    })
  }

  const mediaURL = (path: string) => media.get(path)

  // end is the thread's process going: its reply stops, its queue goes and nothing waits any more.
  function end(id: string) {
    replying.get(id)?.()
    replying.delete(id)
    queued.delete(id)
    setPending(id, null)
    if (logs.has(id)) append(id, { kind: 'exit' })
  }

  // start logs the commands Claude lists once it runs, a moment after its thread starts.
  const start = (id: string) => setTimeout(() => append(id, { kind: 'commands', input: commands }), 1500)

  // breakLog makes a thread's history fail to read, as a chat log the app cannot open does.
  const breakLog = (id: string) => unreadable.add(id)

  return { backend, seed, ask, fork, forget, end, start, breakLog, mediaURL }
}
