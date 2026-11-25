import type { ChatEvent, EventMap, Pending, State } from './types'

type Emit = <K extends keyof EventMap>(event: K, payload: EventMap[K]) => void

// What the mock knows of a thread: whether it has ended, and a way to move its row's state as hooks would.
export type MockThread = { ended: boolean; setState(state: State): void }

type Draft = Omit<ChatEvent, 'seq' | 'at'> & { at?: number }

const minute = 60_000
const replies = [
  'Done. The checkout route now loads its own data, and the cart page no longer calls the cart API twice.',
  'I read the router and the checkout page. Nothing else needs to change for this.',
  'Tests pass: 48 passed, 0 failed.',
]

// mockChat stands in for the chat service: logs kept in memory, chat:<id> events for watched threads, and a reply
// streamed as deltas over a second. thread throws for an id that has no chat.
export function mockChat(emit: Emit, thread: (id: string) => MockThread) {
  const logs = new Map<string, ChatEvent[]>()
  const watched = new Set<string>()
  const pending = new Map<string, Pending>()
  const queued = new Map<string, { id: string; text: string }[]>()
  const replying = new Map<string, () => void>() // stops the reply each thread is streaming
  let nextReply = 0

  const logOf = (id: string) => logs.get(id) ?? []

  function append(id: string, draft: Draft): ChatEvent {
    const log = logOf(id)
    const at = draft.at ?? Date.now()
    const e: ChatEvent = { ...draft, at, seq: draft.kind === 'delta' ? 0 : (log.at(-1)?.seq ?? 0) + 1 }
    if (e.seq > 0) logs.set(id, [...log, e])
    if (watched.has(id)) emit(`chat:${id}` as `chat:${string}/${string}`, { id, events: [{ ...e }] })
    return e
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

  // reply streams a reply over a second, then ends the turn, or runs the next queued message as its own turn.
  function reply(id: string) {
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
      append(id, { kind: 'message', id: message, text })
      const next = queued.get(id)?.shift()
      if (next) {
        append(id, { kind: 'user', id: next.id, text: next.text })
        reply(id)
        return
      }
      append(id, { kind: 'turn_end', cost: 0.04 })
      t.setState('idle')
    }, 1000 / words.length)
    replying.set(id, () => clearInterval(timer))
  }

  // stop ends the turn that runs, as an interrupt does.
  function stop(id: string) {
    const running = replying.get(id)
    if (!running && !pending.has(id)) return
    running?.()
    replying.delete(id)
    setPending(id, null)
    append(id, { kind: 'turn_end', error: true, cost: 0.04 })
    thread(id).setState('idle')
  }

  // seed fills a thread's log with two turns, the second of which waits on a Bash approval.
  function seed(id: string) {
    const ago = (minutes: number) => Date.now() - minutes * minute
    const turns: Draft[] = [
      { kind: 'user', id: 'u-1', text: 'Move checkout to its own route, and keep the cart page as it is.', at: ago(38) },
      { kind: 'init', id: 'mock-session', at: ago(38) },
      { kind: 'message', id: 'msg_1', text: "I'll read the router first.", at: ago(37) },
      { kind: 'tool_use', id: 'toolu_1', tool: 'Read', input: { file_path: 'app-frontend/src/js/routes.tsx' }, at: ago(37) },
      { kind: 'tool_result', parent: 'toolu_1', text: "export const routes = [\n  { path: '/', page: Home },\n  { path: '/cart', page: Cart },\n]", at: ago(37) },
      { kind: 'tool_use', id: 'toolu_2', tool: 'Edit', input: { file_path: 'app-frontend/src/js/routes.tsx', old_string: "  { path: '/cart', page: Cart },", new_string: "  { path: '/cart', page: Cart },\n  { path: '/checkout', page: Checkout }," }, at: ago(36) },
      { kind: 'tool_result', parent: 'toolu_2', text: 'The file was updated.', at: ago(36) },
      { kind: 'message', id: 'msg_2', text: 'Checkout now has its own route at `/checkout`. The cart page is unchanged.', at: ago(35) },
      { kind: 'turn_end', cost: 0.21, at: ago(35) },
      { kind: 'user', id: 'u-2', text: 'Run the tests, and install whatever they need.', at: ago(5) },
      { kind: 'init', id: 'mock-session', at: ago(5) },
      { kind: 'message', id: 'msg_3', text: 'The tests need a package that is not installed yet.', at: ago(4) },
      { kind: 'tool_use', id: 'toolu_3', tool: 'Bash', input: { command: 'npm install', description: 'Install the missing packages' }, at: ago(4) },
      { kind: 'approval', id: 'req-1', tool: 'Bash', input: { command: 'npm install', description: 'Install the missing packages' }, at: ago(4) },
    ]
    turns.forEach((e) => append(id, e))
    setPending(id, { id, request: 'req-1', tool: 'Bash', summary: 'npm install', kind: 'tool' })
  }

  function answer(id: string, request: string, allow: boolean, message: string) {
    const t = live(id)
    const p = pending.get(id)
    if (p?.request !== request) throw `Thread ${id} did not take the answer`
    append(id, { kind: 'answer', id: request, text: allow ? 'allow' : 'deny' })
    setPending(id, null)
    const tool = [...logOf(id)].reverse().find((e) => e.kind === 'tool_use')
    append(id, { kind: 'tool_result', parent: tool?.id, text: allow ? 'added 12 packages in 3s' : message || 'The user denied this tool.', error: !allow })
    t.setState('working')
    reply(id)
  }

  const backend = {
    ChatHistory: async (id: string, after: number) => {
      thread(id)
      return logOf(id).filter((e) => e.seq > after).map((e) => ({ ...e }))
    },
    ChatWatch: async (id: string) => {
      live(id)
      watched.add(id)
    },
    ChatUnwatch: async (id: string) => {
      watched.delete(id)
    },
    ChatSend: async (id: string, text: string) => {
      const t = live(id)
      const message = crypto.randomUUID()
      const waiting = pending.get(id)
      if (waiting) {
        append(id, { kind: 'answer', id: waiting.request, text: 'deny' })
        append(id, { kind: 'user', id: message, text, parent: waiting.request })
        setPending(id, null)
        t.setState('working')
        reply(id)
        return message
      }
      if (replying.has(id)) {
        append(id, { kind: 'queued', id: message, text })
        queued.set(id, [...(queued.get(id) ?? []), { id: message, text }])
        return message
      }
      append(id, { kind: 'user', id: message, text })
      append(id, { kind: 'init', id: 'mock-session' })
      t.setState('working')
      reply(id)
      return message
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
      if (!queued.get(id)?.some((m) => m.id === message)) throw `Thread ${id} did not take Send now`
      append(id, { kind: 'interrupt', id: message })
      stop(id)
      const next = queued.get(id)?.shift()
      if (!next) return
      append(id, { kind: 'user', id: next.id, text: next.text })
      thread(id).setState('working')
      reply(id)
    },
    ChatAnswer: async (id: string, request: string, allow: boolean, message: string) => answer(id, request, allow, message),
    ChatAnswerInput: async (id: string, request: string, input: string) => {
      try {
        JSON.parse(input)
      } catch {
        throw 'The answer is not valid JSON'
      }
      answer(id, request, true, '')
    },
    ChatPending: async () => [...pending.values()].map((p) => ({ ...p })),
  }

  return { backend, seed }
}
