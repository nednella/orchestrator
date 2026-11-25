import type { ChatEvent, Pending } from './types'

// Command is one slash command, as Claude lists them in a commands event.
export type Command = { name: string; description: string; argumentHint: string; builtin?: boolean; aliases?: string[] }

export type ToolCall = { id: string; tool: string; input: unknown; result: string | null; error: boolean; answer?: 'allow' | 'deny' }

// Ask is a question Claude puts to the user, or a plan it asks the user to approve. request is the approval that
// answers it, '' until Claude asks for one.
export type Ask = { key: string; at: number; id: string; input: unknown; request: string; answer?: 'allow' | 'deny'; expired: boolean; result: string | null }

export type TimelineItem =
  | { kind: 'user'; key: string; at: number; id: string; text: string; files: string[] }
  | { kind: 'reply'; key: string; at: number; id: string; text: string; streaming: boolean }
  | { kind: 'work'; key: string; startedAt: number; endedAt: number | null; calls: ToolCall[]; notes: string[] }
  | { kind: 'approval'; key: string; at: number; request: string; tool: string; input: unknown; expired: boolean }
  | ({ kind: 'question' } & Ask)
  | ({ kind: 'plan' } & Ask)
  | { kind: 'notice'; key: string; at: number; text: string; error: boolean }

export type QueuedMessage = { id: string; text: string; files: string[] }

export type Timeline = { items: TimelineItem[]; queued: QueuedMessage[]; turnOpen: boolean; pending: Pending | null; commands: Command[] }

type Notice = Extract<TimelineItem, { kind: 'notice' }>
type ApprovalItem = Extract<TimelineItem, { kind: 'approval' }>
type AskItem = Extract<TimelineItem, { kind: 'question' | 'plan' }>
type Reply = { kind: 'reply'; key: string; at: number; id: string; blocks: string[] }
// A part is one thing a turn holds, in the order it came.
type Part = Reply | { kind: 'call'; call: ToolCall } | AskItem | ApprovalItem
// An approval answers an ask, or else shows as an item of its own, next to the call it is for when there is one.
type Approval = { request: string; tool: string; ask?: AskItem; call?: ToolCall; item?: ApprovalItem; answered: boolean }
type Turn = { key: string; startedAt: number; parts: Part[]; approvals: Approval[] }

const askKinds: Record<string, 'question' | 'plan'> = { AskUserQuestion: 'question', ExitPlanMode: 'plan' }

// buildTimeline turns a thread's logged events, and the text its messages have streamed so far, into what its chat
// shows. thread is the thread's id, for the pending approval.
export function buildTimeline(events: ChatEvent[], live: Record<string, string>, thread = ''): Timeline {
  const items: TimelineItem[] = []
  const calls = new Map<string, ToolCall>()
  const asks = new Map<string, AskItem>()
  const approvals = new Map<string, Approval>()
  const requested = new Set<ToolCall>()
  const queued = new Map<string, QueuedMessage>()
  const deliveredAt = new Map<string, number>()
  events.forEach((e, i) => {
    if (e.kind === 'user' && e.id) deliveredAt.set(e.id, i)
  })
  let commands: Command[] = []
  let turn: Turn | null = null
  let interrupted = false

  const open = (e: ChatEvent): Turn => (turn ??= { key: (e.kind === 'user' && e.id) || `seq-${e.seq}`, startedAt: e.at, parts: [], approvals: [] })

  // close folds the turn into the items and gives back the approvals it left unanswered.
  const close = (endedAt: number): Approval[] => {
    if (!turn) return []
    const left = turn.approvals.filter((a) => !a.answered)
    items.push(...folded(turn, endedAt))
    turn = null
    return left
  }

  // end closes the turn for good: what it left unanswered can no longer be answered, and a queued message Claude will
  // not take in later is shown as not sent.
  const end = (e: ChatEvent, index: number, notice: Omit<Notice, 'kind' | 'key' | 'at'> | null, dropQueued: boolean) => {
    for (const a of turn?.approvals ?? []) {
      if (a.answered) continue
      if (a.ask) a.ask.expired = true
      if (a.item) a.item.expired = true
    }
    close(e.at)
    if (notice) items.push({ kind: 'notice', key: `notice-${e.seq}`, at: e.at, ...notice })
    if (!dropQueued) return
    for (const q of queued.values()) {
      if ((deliveredAt.get(q.id) ?? -1) > index) continue
      queued.delete(q.id)
      items.push({ kind: 'notice', key: `unsent-${q.id}`, at: e.at, text: `Not sent: ${firstLine(q.text)}`, error: false })
    }
  }

  events.forEach((e, i) => {
    switch (e.kind) {
      case 'user': {
        // A message Claude takes in while it works splits the turn there.
        const left = close(e.at)
        if (e.id) queued.delete(e.id)
        items.push({ kind: 'user', key: `user-${e.id || e.seq}`, at: e.at, id: e.id ?? '', text: e.text ?? '', files: e.files ?? [] })
        open(e).approvals.push(...left)
        return
      }
      case 'queued':
        if (e.id) queued.set(e.id, { id: e.id, text: e.text ?? '', files: e.files ?? [] })
        return
      case 'message': {
        const t = open(e)
        const id = e.id || `seq-${e.seq}`
        const reply = t.parts.find((p): p is Reply => p.kind === 'reply' && p.id === id)
        if (reply) reply.blocks.push(e.text ?? '')
        else t.parts.push({ kind: 'reply', key: `reply-${id}`, at: e.at, id, blocks: [e.text ?? ''] })
        return
      }
      case 'tool_use': {
        const t = open(e)
        const id = e.id || `seq-${e.seq}`
        const kind = askKinds[e.tool ?? '']
        if (kind) {
          const ask: AskItem = { kind, key: `${kind}-${id}`, at: e.at, id, input: e.input, request: '', expired: false, result: null }
          asks.set(id, ask)
          t.parts.push(ask)
          return
        }
        const call: ToolCall = { id, tool: e.tool ?? '', input: e.input, result: null, error: false }
        calls.set(id, call)
        t.parts.push({ kind: 'call', call })
        return
      }
      case 'tool_result': {
        const call = calls.get(e.parent ?? '')
        if (call) Object.assign(call, { result: e.text ?? '', error: e.error ?? false })
        const ask = asks.get(e.parent ?? '')
        if (ask) ask.result = e.text ?? ''
        return
      }
      case 'approval': {
        const t = open(e)
        const a: Approval = { request: e.id ?? '', tool: e.tool ?? '', answered: false }
        const kind = askKinds[a.tool]
        // The approval names the tool, not the call: it is for the latest call of that tool still waiting on one.
        if (kind) a.ask = [...asks.values()].reverse().find((x) => x.kind === kind && x.request === '' && x.result === null)
        else a.call = [...calls.values()].reverse().find((c) => c.tool === a.tool && c.result === null && !requested.has(c))
        if (a.ask) a.ask.request = a.request
        else {
          if (a.call) requested.add(a.call)
          a.item = { kind: 'approval', key: `approval-${a.request}`, at: e.at, request: a.request, tool: a.tool, input: e.input, expired: false }
          t.parts.push(a.item)
        }
        approvals.set(a.request, a)
        t.approvals.push(a)
        return
      }
      case 'answer': {
        const a = approvals.get(e.id ?? '')
        if (!a || a.answered) return
        const answer = e.text === 'allow' ? 'allow' : 'deny'
        a.answered = true
        if (a.ask) a.ask.answer = answer
        if (a.call) a.call.answer = answer
        if (turn && a.item) turn.parts = turn.parts.filter((p) => p !== a.item)
        return
      }
      case 'interrupt':
        if (turn) interrupted = true
        return
      case 'turn_end': {
        const stopped = interrupted
        interrupted = false
        end(e, i, e.error ? { text: stopped ? 'Stopped' : 'The turn ended with an error', error: !stopped } : null, stopped)
        return
      }
      case 'exit':
        interrupted = false
        end(e, i, { text: 'Claude stopped', error: e.error ?? false }, true)
        return
      case 'commands':
        commands = Array.isArray(e.input) ? e.input.filter((c): c is Command => typeof c?.name === 'string') : []
        return
    }
  })

  const running = turn as Turn | null
  if (running) items.push(...unfolded(running, live, events.at(-1)?.at ?? 0))
  const waiting = running?.approvals.find((a) => !a.answered)
  return {
    items,
    queued: [...queued.values()],
    turnOpen: running !== null,
    pending: waiting ? pendingOf(thread, waiting) : null,
    commands,
  }
}

// folded is a finished turn. When it made tool calls, they and every reply but the last go into one work item.
function folded(turn: Turn, endedAt: number): TimelineItem[] {
  const calls = turn.parts.flatMap((p) => (p.kind === 'call' ? [p.call] : []))
  if (calls.length === 0) return turn.parts.map((p) => (p.kind === 'reply' ? replyItem(p, '') : (p as AskItem | ApprovalItem)))
  const replies = turn.parts.filter((p): p is Reply => p.kind === 'reply')
  const rest = turn.parts.filter((p): p is AskItem | ApprovalItem => p.kind !== 'call' && p.kind !== 'reply')
  const last = replies.at(-1)
  const notes = replies.slice(0, -1).map((r) => r.blocks.join('\n\n'))
  return [{ kind: 'work', key: `work-${turn.key}`, startedAt: turn.startedAt, endedAt, calls, notes }, ...rest, ...(last ? [replyItem(last, '')] : [])]
}

// unfolded is the turn that runs: its parts in order, each run of calls as one work item, then the messages that have
// only streamed so far.
function unfolded(turn: Turn, live: Record<string, string>, now: number): TimelineItem[] {
  const out: TimelineItem[] = []
  for (const p of turn.parts) {
    if (p.kind !== 'call') {
      out.push(p.kind === 'reply' ? replyItem(p, live[p.id] ?? '') : p)
      continue
    }
    const work = out.at(-1)
    if (work?.kind === 'work') work.calls.push(p.call)
    else out.push({ kind: 'work', key: `work-${p.call.id}`, startedAt: turn.startedAt, endedAt: null, calls: [p.call], notes: [] })
  }
  for (const [id, text] of Object.entries(live)) {
    if (!turn.parts.some((p) => p.kind === 'reply' && p.id === id)) out.push({ kind: 'reply', key: `reply-${id}`, at: now, id, text, streaming: true })
  }
  return out
}

function replyItem(r: Reply, streamed: string): TimelineItem {
  return { kind: 'reply', key: r.key, at: r.at, id: r.id, text: [...r.blocks, ...(streamed ? [streamed] : [])].join('\n\n'), streaming: streamed !== '' }
}

// pendingOf describes the approval a thread waits on, as the chat service does.
function pendingOf(thread: string, a: Approval): Pending {
  const input = (a.ask?.input ?? a.item?.input ?? {}) as { command?: string; file_path?: string; path?: string; pattern?: string; url?: string; plan?: string; questions?: { question?: string }[] }
  const kind = askKinds[a.tool] ?? 'tool'
  let summary = ''
  if (kind === 'question') summary = input.questions?.[0]?.question ?? ''
  else if (kind === 'plan') summary = firstLine(input.plan ?? '')
  else summary = firstLine([input.command, input.file_path, input.path, input.pattern, input.url].find((s) => typeof s === 'string' && s !== '') ?? '')
  return { id: thread, request: a.request, tool: a.tool, summary, kind }
}

function firstLine(s: string): string {
  return (
    s
      .split('\n')
      .map((line) => line.trim())
      .find((line) => line !== '') ?? ''
  )
}
