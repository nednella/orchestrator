import type { ChatEvent, Pending } from './types'

// Command is one slash command, as Claude lists them in a commands event.
export type Command = { name: string; description: string; argumentHint: string; builtin?: boolean; aliases?: string[] }

// A step is one thing a fold holds, in the order it came: a tool call, or text Claude wrote between calls.
export type WorkStep = { kind: 'call'; call: ToolCall } | { kind: 'note'; key: string; text: string }

export type ToolCall = { id: string; tool: string; input: unknown; result: string | null; error: boolean; answer?: 'allow' | 'deny' }

// Ask is a question Claude puts to the user, or a plan it asks the user to approve. request is the approval that
// answers it, '' until Claude asks for one. answers is what a question was answered with, by question, once known.
export type Ask = { key: string; at: number; id: string; input: unknown; request: string; answer?: 'allow' | 'deny'; answers: Record<string, string> | null; expired: boolean; result: string | null }

export type TimelineItem =
  | { kind: 'user'; key: string; at: number; id: string; text: string; files: string[] }
  | { kind: 'reply'; key: string; at: number; id: string; text: string; streaming: boolean }
  | { kind: 'work'; key: string; startedAt: number; endedAt: number | null; calls: ToolCall[]; steps: WorkStep[] }
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
// unsent holds the not-sent notices logged during the turn: the host drops a message before the turn ends, and the
// notice shows after the turn's own.
type Turn = { key: string; startedAt: number; parts: Part[]; approvals: Approval[]; unsent: Notice[] }

const askKinds: Record<string, 'question' | 'plan'> = { AskUserQuestion: 'question', ExitPlanMode: 'plan' }

// approvalKind is what an approval for the tool asks of the user: to let a tool run, to answer a question or to
// approve a plan.
export const approvalKind = (tool: string): Pending['kind'] => askKinds[tool] ?? 'tool'

const answersOf = (input: unknown): Record<string, string> | null => {
  const answers = (input as { answers?: unknown } | null)?.answers
  return answers && typeof answers === 'object' ? (answers as Record<string, string>) : null
}

// buildTimeline turns a thread's logged events, and the text its messages have streamed so far, into what its chat
// shows. thread is the thread's id, for the pending approval.
export function buildTimeline(events: ChatEvent[], live: Record<string, string>, thread = ''): Timeline {
  const items: TimelineItem[] = []
  const calls = new Map<string, ToolCall>()
  const asks = new Map<string, AskItem>()
  const approvals = new Map<string, Approval>()
  const requested = new Set<ToolCall>()
  const queued = new Map<string, QueuedMessage>()
  let commands: Command[] = []
  let turn: Turn | null = null
  let interrupted = false

  const open = (e: ChatEvent): Turn => (turn ??= { key: (e.kind === 'user' && e.id) || `seq-${e.seq}`, startedAt: e.at, parts: [], approvals: [], unsent: [] })

  const unsent = (id: string, text: string, at: number): Notice => {
    queued.delete(id)
    return { kind: 'notice', key: `unsent-${id}`, at, text: `Not sent: ${firstLine(text)}`, error: false }
  }

  // close folds the turn into the items, then its notice and its not-sent notices, and gives back the approvals it
  // left unanswered.
  const close = (endedAt: number, notice: Notice | null = null): Approval[] => {
    const t = turn as Turn | null
    turn = null
    if (t) items.push(...folded(t, endedAt))
    if (notice) items.push(notice)
    if (t) items.push(...t.unsent)
    return t?.approvals.filter((a) => !a.answered) ?? []
  }

  // end closes the turn for good: what it left unanswered can no longer be answered.
  const end = (e: ChatEvent, notice: Omit<Notice, 'kind' | 'key' | 'at'> | null) => {
    for (const a of turn?.approvals ?? []) {
      if (a.answered) continue
      if (a.ask) a.ask.expired = true
      if (a.item) a.item.expired = true
    }
    close(e.at, notice && { kind: 'notice', key: `notice-${e.seq}`, at: e.at, ...notice })
  }

  events.forEach((e) => {
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
      case 'unsent': {
        if (!e.id || !queued.has(e.id)) return
        const notice = unsent(e.id, queued.get(e.id)?.text ?? '', e.at)
        if (turn) turn.unsent.push(notice)
        else items.push(notice)
        return
      }
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
          const ask: AskItem = { kind, key: `${kind}-${id}`, at: e.at, id, input: e.input, request: '', answers: null, expired: false, result: null }
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
        // An approval in an older log names only the tool: it is for the turn's earliest call of it still waiting on one.
        if (kind) a.ask = e.parent ? asks.get(e.parent) : t.parts.find((p): p is AskItem => p.kind === kind && p.request === '' && p.result === null)
        else a.call = e.parent ? calls.get(e.parent) : t.parts.flatMap((p) => (p.kind === 'call' ? [p.call] : [])).find((c) => c.tool === a.tool && c.result === null && !requested.has(c))
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
        if (a.ask) Object.assign(a.ask, { answer, answers: answersOf(e.input) })
        if (a.call) a.call.answer = answer
        if (turn && a.item) turn.parts = turn.parts.filter((p) => p !== a.item)
        return
      }
      case 'interrupt':
        if (turn) interrupted = true
        return
      case 'turn_end':
        end(e, e.error ? { text: interrupted ? 'Stopped' : 'The turn ended with an error', error: !interrupted } : null)
        interrupted = false
        return
      case 'exit':
        interrupted = false
        end(e, { text: 'Claude stopped', error: e.error ?? false })
        // Claude is gone, so a queued message it did not take in will not be sent.
        for (const q of queued.values()) items.push(unsent(q.id, q.text, e.at))
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
  const steps = turn.parts.flatMap((p): WorkStep[] => {
    if (p.kind === 'call') return [{ kind: 'call', call: p.call }]
    if (p.kind === 'reply' && p !== last) return [{ kind: 'note', key: p.key, text: p.blocks.join('\n\n') }]
    return []
  })
  return [{ kind: 'work', key: `work-${turn.key}`, startedAt: turn.startedAt, endedAt, calls, steps }, ...rest, ...(last ? [replyItem(last, '')] : [])]
}

// unfolded is the turn that runs: its parts in order, each run of calls as one work item, then the messages that have
// only streamed so far. A call that waits on its approval shows only as the approval's card.
function unfolded(turn: Turn, live: Record<string, string>, now: number): TimelineItem[] {
  const out: TimelineItem[] = []
  const asked = new Set(turn.approvals.filter((a) => !a.answered && a.item).map((a) => a.call))
  for (const p of turn.parts) {
    if (p.kind !== 'call') {
      out.push(p.kind === 'reply' ? replyItem(p, live[p.id] ?? '') : p)
      continue
    }
    if (asked.has(p.call)) continue
    const work = out.at(-1)
    if (work?.kind === 'work') {
      work.calls.push(p.call)
      work.steps.push({ kind: 'call', call: p.call })
    } else out.push({ kind: 'work', key: `work-${p.call.id}`, startedAt: turn.startedAt, endedAt: null, calls: [p.call], steps: [{ kind: 'call', call: p.call }] })
  }
  for (const [id, text] of Object.entries(live)) {
    if (!turn.parts.some((p) => p.kind === 'reply' && p.id === id)) out.push({ kind: 'reply', key: `reply-${id}`, at: now, id, text, streaming: true })
  }
  return [...out, ...turn.unsent]
}

function replyItem(r: Reply, streamed: string): TimelineItem {
  return { kind: 'reply', key: r.key, at: r.at, id: r.id, text: [...r.blocks, ...(streamed ? [streamed] : [])].join('\n\n'), streaming: streamed !== '' }
}

// pendingOf describes the approval a thread waits on, as the chat service does.
function pendingOf(thread: string, a: Approval): Pending {
  const input = (a.ask?.input ?? a.item?.input ?? {}) as { command?: string; file_path?: string; path?: string; pattern?: string; url?: string; plan?: string; questions?: { question?: string }[] }
  const kind = approvalKind(a.tool)
  let summary = ''
  if (kind === 'question') summary = input.questions?.[0]?.question ?? ''
  else if (kind === 'plan') summary = firstLine(input.plan ?? '')
  else summary = firstLine([input.command, input.file_path, input.path, input.pattern, input.url].find((s) => typeof s === 'string' && s !== '') ?? '')
  return { id: thread, request: a.request, tool: a.tool, summary, kind }
}

export function firstLine(s: string): string {
  return (
    s
      .split('\n')
      .map((line) => line.trim())
      .find((line) => line !== '') ?? ''
  )
}
