import type { Access, ChatEvent, Pending } from './types'

// Command is one slash command, as Claude lists them in a commands event.
export type Command = { name: string; description: string; argumentHint: string; builtin?: boolean; aliases?: string[] }

// A step is one thing a fold holds, in the order it came: a tool call, text Claude wrote between calls, or a thinking
// block, live while it streams.
export type WorkStep = { kind: 'call'; call: ToolCall } | { kind: 'note'; key: string; text: string } | { kind: 'thought'; key: string; text: string; live: boolean }

export type ToolCall = { id: string; tool: string; input: unknown; result: string | null; error: boolean; answer?: 'allow' | 'deny' }

// Ask is a question Claude puts to the user, or a plan it asks the user to approve. request is the approval that
// answers it, '' until Claude asks for one. answers is what a question was answered with, by question, once known.
export type Ask = { key: string; at: number; id: string; input: unknown; request: string; answer?: 'allow' | 'deny'; answers: Record<string, string> | null; expired: boolean; result: string | null }

export type TimelineItem =
  // forkAt is the uuid of the reply before the message, which a fork from the message continues from; '' when none or
  // when the reply is from before the last compaction.
  | { kind: 'user'; key: string; at: number; id: string; text: string; files: string[]; forkAt: string }
  // uuid is that of the reply's last logged block, which a fork from the reply ends at; '' when none or when the reply
  // is from before the last compaction. at is when its last block came. ended is set on the reply a turn stopped or
  // failed on: the turn's last part.
  | { kind: 'reply'; key: string; at: number; id: string; uuid: string; text: string; streaming: boolean; ended?: TurnEnd }
  // header is whether the work item heads its turn and shows how long the turn has worked: the first item of the turn
  // that runs, and the fold of a finished one. In the turn that runs, the calls and thoughts after a reply or a card
  // are a work item that does not head it.
  | { kind: 'work'; key: string; header: boolean; startedAt: number; endedAt: number | null; calls: ToolCall[]; steps: WorkStep[] }
  | { kind: 'approval'; key: string; at: number; request: string; tool: string; input: unknown; expired: boolean }
  | ({ kind: 'question' } & Ask)
  | ({ kind: 'plan' } & Ask)
  | { kind: 'notice'; key: string; at: number; text: string; error: boolean }
  // restart is the thread starting again with a new access, which the owner asked for.
  | { kind: 'restart'; key: string; at: number; access: Access }

// TurnEnd is how a turn ended that did not finish: stopped by the owner or Claude's exit, or failed.
export type TurnEnd = 'stopped' | 'failed'

export type QueuedMessage = { id: string; text: string; files: string[] }

export type Timeline = { items: TimelineItem[]; queued: QueuedMessage[]; turnOpen: boolean; pending: Pending | null; commands: Command[] }

type Notice = Extract<TimelineItem, { kind: 'notice' }>
type Restart = Extract<TimelineItem, { kind: 'restart' }>
type ApprovalItem = Extract<TimelineItem, { kind: 'approval' }>
type AskItem = Extract<TimelineItem, { kind: 'question' | 'plan' }>
type Reply = { kind: 'reply'; key: string; at: number; id: string; uuid: string; blocks: string[] }
type ReplyItem = Extract<TimelineItem, { kind: 'reply' }>
type Thought = { kind: 'thought'; key: string; id: string; text: string }
// A part is one thing a turn holds, in the order it came.
type Part = Reply | Thought | { kind: 'call'; call: ToolCall } | AskItem | ApprovalItem
// An approval answers an ask, or else shows as an item of its own, next to the call it is for when there is one.
type Approval = { request: string; tool: string; ask?: AskItem; call?: ToolCall; item?: ApprovalItem; answered: boolean }
// unsent holds the not-sent notices logged during the turn: the host drops a message before the turn ends, and the
// notice shows after the turn's own.
type Turn = { key: string; startedAt: number; parts: Part[]; approvals: Approval[]; unsent: Notice[] }

// One empty list for every message without images keeps a memoised row from rendering again.
const noFiles: string[] = []

// turnError is why Claude says a turn failed, without the diagnostic lines it adds for its own use; '' when none.
const turnError = (errors = '') => errors.split('\n').filter((line) => !line.startsWith('[ede_diagnostic]')).join('\n').trim()

const askKinds: Record<string, 'question' | 'plan'> = { AskUserQuestion: 'question', ExitPlanMode: 'plan' }

// approvalKind is what an approval for the tool asks of the user: to let a tool run, to answer a question or to
// approve a plan.
export const approvalKind = (tool: string): Pending['kind'] => askKinds[tool] ?? 'tool'

const answersOf = (input: unknown): Record<string, string> | null => {
  const answers = (input as { answers?: unknown } | null)?.answers
  return answers && typeof answers === 'object' ? (answers as Record<string, string>) : null
}

// buildTimeline turns a thread's logged events, and the text its messages and their thinking have streamed so far,
// into what its chat shows. thread is the thread's id, for the pending approval.
export function buildTimeline(events: ChatEvent[], live: Record<string, string>, thread = '', thinking: Record<string, string> = {}): Timeline {
  const items: TimelineItem[] = []
  const calls = new Map<string, ToolCall>()
  const asks = new Map<string, AskItem>()
  const approvals = new Map<string, Approval>()
  const requested = new Set<ToolCall>()
  const queued = new Map<string, QueuedMessage>()
  let commands: Command[] = []
  let turn: Turn | null = null
  let interrupted = false
  // Claude resumes a compacted conversation from its last compaction and cannot find a message before it, so no fork
  // ends at one.
  const compacted = events.findLastIndex((e) => e.kind === 'compact')

  const open = (e: ChatEvent): Turn => (turn ??= { key: (e.kind === 'user' && e.id) || `seq-${e.seq}`, startedAt: e.at, parts: [], approvals: [], unsent: [] })

  const unsent = (id: string, text: string, at: number, error = false): Notice => {
    queued.delete(id)
    return { kind: 'notice', key: `unsent-${id}`, at, text: `Not sent: ${firstLine(text)}`, error }
  }

  // close folds the turn into the items, then its notice and its not-sent notices, and gives back the approvals it
  // left unanswered.
  const close = (endedAt: number, notice: Notice | Restart | null = null, ended?: TurnEnd): Approval[] => {
    const t = turn as Turn | null
    turn = null
    if (t) items.push(...folded(t, endedAt, ended))
    if (notice) items.push(notice)
    if (t) items.push(...t.unsent)
    return t?.approvals.filter((a) => !a.answered) ?? []
  }

  // end closes the turn for good: what it left unanswered can no longer be answered.
  const end = (e: ChatEvent, notice: Notice | Restart | null, ended?: TurnEnd) => {
    for (const a of turn?.approvals ?? []) {
      if (a.answered) continue
      if (a.ask) a.ask.expired = true
      if (a.item) a.item.expired = true
    }
    close(e.at, notice, ended)
  }

  events.forEach((e, i) => {
    switch (e.kind) {
      case 'user': {
        // A message Claude takes in while it works splits the turn there.
        const left = close(e.at)
        if (e.id) queued.delete(e.id)
        const forkAt = items.findLast((i): i is ReplyItem => i.kind === 'reply')?.uuid ?? ''
        items.push({ kind: 'user', key: `user-${e.id || e.seq}`, at: e.at, id: e.id ?? '', text: e.text ?? '', files: e.files ?? noFiles, forkAt })
        open(e).approvals.push(...left)
        return
      }
      case 'queued':
        if (e.id) queued.set(e.id, { id: e.id, text: e.text ?? '', files: e.files ?? [] })
        return
      // An unsent error is a message a Claude that failed at its start never read, sent or queued.
      case 'unsent': {
        if (!e.id || (!queued.has(e.id) && !e.error)) return
        const notice = unsent(e.id, queued.get(e.id)?.text ?? e.text ?? '', e.at, e.error)
        if (turn) turn.unsent.push(notice)
        else items.push(notice)
        return
      }
      case 'message': {
        const t = open(e)
        const id = e.id || `seq-${e.seq}`
        const uuid = i > compacted ? (e.uuid ?? '') : ''
        const reply = t.parts.find((p): p is Reply => p.kind === 'reply' && p.id === id)
        if (reply) {
          reply.blocks.push(e.text ?? '')
          reply.uuid = uuid || reply.uuid
          reply.at = e.at
        } else t.parts.push({ kind: 'reply', key: `reply-${id}`, at: e.at, id, uuid, blocks: [e.text ?? ''] })
        return
      }
      case 'thought': {
        const t = open(e)
        const id = e.id || `seq-${e.seq}`
        t.parts.push({ kind: 'thought', key: thoughtKey(t, id), id, text: e.text ?? '' })
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
        end(e, e.error ? noticeOf(e, interrupted ? 'Stopped' : turnError(e.text) || 'The turn ended with an error', !interrupted) : null, e.error ? (interrupted ? 'stopped' : 'failed') : undefined)
        interrupted = false
        return
      case 'exit':
        interrupted = false
        end(e, exitItem(e), e.error ? 'failed' : 'stopped')
        // Claude is gone, so a queued message it did not take in will not be sent.
        for (const q of queued.values()) items.push(unsent(q.id, q.text, e.at))
        return
      // A run whose host was killed logged no exit: its turn and what it waits on end where the next run starts. A start
      // with text is the same run's Claude started again without a flag it did not know, before it read anything.
      case 'start':
        if (e.text) return
        interrupted = false
        end(e, null, 'stopped')
        return
      case 'commands':
        commands = Array.isArray(e.input) ? e.input.filter((c): c is Command => typeof c?.name === 'string') : []
        return
    }
  })

  const running = turn as Turn | null
  if (running) items.push(...unfolded(running, live, thinking, events.at(-1)?.at ?? 0))
  const waiting = running?.approvals.find((a) => !a.answered)
  return {
    items,
    queued: [...queued.values()],
    turnOpen: running !== null,
    pending: waiting ? pendingOf(thread, waiting) : null,
    commands,
  }
}

function noticeOf(e: ChatEvent, text: string, error: boolean): Notice {
  return { kind: 'notice', key: `notice-${e.seq}`, at: e.at, text, error }
}

// exitItem is what the chat shows where a run of Claude ended: a quiet line for a settle or a restart the host made
// itself, and Claude's own failure in red.
function exitItem(e: ChatEvent): Notice | Restart {
  if (e.stop === 'restart') return { kind: 'restart', key: `notice-${e.seq}`, at: e.at, access: e.text as Access }
  return noticeOf(e, e.stop === 'settle' ? 'Settled' : 'Claude stopped', e.error ?? false)
}

// thoughtKey is the key of the next thought of message id in the turn, the same while it streams and once logged.
const thoughtKey = (turn: Turn, id: string) => `thought-${id}-${turn.parts.filter((p) => p.kind === 'thought' && p.id === id).length}`

// folded is a finished turn. When it made tool calls or thought, they and every reply but the last go into one work
// item. A turn that ended unfinished marks its last part, when that is a reply.
function folded(turn: Turn, endedAt: number, ended?: TurnEnd): TimelineItem[] {
  const calls = turn.parts.flatMap((p) => (p.kind === 'call' ? [p.call] : []))
  const endedOn = turn.parts.at(-1)
  const reply = (r: Reply) => replyItem(r, '', r === endedOn ? ended : undefined)
  const work = (steps: WorkStep[]): TimelineItem => ({ kind: 'work', key: `work-${turn.key}`, header: true, startedAt: turn.startedAt, endedAt, calls, steps })
  // A turn with nothing to fold keeps its heading in place, so the timeline does not move when it ends.
  if (!turn.parts.some((p) => p.kind === 'call' || p.kind === 'thought')) return [work([]), ...turn.parts.map((p) => (p.kind === 'reply' ? reply(p) : (p as AskItem | ApprovalItem)))]
  const replies = turn.parts.filter((p): p is Reply => p.kind === 'reply')
  const rest = turn.parts.filter((p): p is AskItem | ApprovalItem => p.kind !== 'call' && p.kind !== 'reply' && p.kind !== 'thought')
  const last = replies.at(-1)
  const steps = turn.parts.flatMap((p): WorkStep[] => {
    if (p.kind === 'call') return [{ kind: 'call', call: p.call }]
    if (p.kind === 'thought') return [{ kind: 'thought', key: p.key, text: p.text, live: false }]
    if (p.kind === 'reply' && p !== last) return [{ kind: 'note', key: p.key, text: p.blocks.join('\n\n') }]
    return []
  })
  return [work(steps), ...rest, ...(last ? [reply(last)] : [])]
}

// unfolded is the turn that runs: the work item that heads it, then its parts in order, each run of calls and thoughts
// as one work item, the first in the heading one, then the thinking and the messages that have only streamed so far. A
// call that waits on its approval shows only as the approval's card.
function unfolded(turn: Turn, live: Record<string, string>, thinking: Record<string, string>, now: number): TimelineItem[] {
  const out: TimelineItem[] = [{ kind: 'work', key: `work-${turn.key}`, header: true, startedAt: turn.startedAt, endedAt: null, calls: [], steps: [] }]
  const asked = new Set(turn.approvals.filter((a) => !a.answered && a.item).map((a) => a.call))
  // line adds a step to the work item the items end with, or to a new one after a reply or a card.
  const line = (step: WorkStep, key: string) => {
    let work = out.at(-1)
    if (work?.kind !== 'work') {
      work = { kind: 'work', key: `work-${key}`, header: false, startedAt: turn.startedAt, endedAt: null, calls: [], steps: [] }
      out.push(work)
    }
    work.steps.push(step)
    if (step.kind === 'call') work.calls.push(step.call)
  }
  for (const p of turn.parts) {
    if (p.kind === 'call') {
      if (!asked.has(p.call)) line({ kind: 'call', call: p.call }, p.call.id)
    } else if (p.kind === 'thought') line({ kind: 'thought', key: p.key, text: p.text, live: false }, p.key)
    else out.push(p.kind === 'reply' ? replyItem(p, live[p.id] ?? '') : p)
  }
  for (const [id, text] of Object.entries(thinking)) {
    const key = thoughtKey(turn, id)
    line({ kind: 'thought', key, text, live: true }, key)
  }
  for (const [id, text] of Object.entries(live)) {
    if (!turn.parts.some((p) => p.kind === 'reply' && p.id === id)) out.push({ kind: 'reply', key: `reply-${id}`, at: now, id, uuid: '', text, streaming: true })
  }
  return [...out, ...turn.unsent]
}

function replyItem(r: Reply, streamed: string, ended?: TurnEnd): TimelineItem {
  return { kind: 'reply', key: r.key, at: r.at, id: r.id, uuid: r.uuid, text: [...r.blocks, ...(streamed ? [streamed] : [])].join('\n\n'), streaming: streamed !== '', ...(ended ? { ended } : {}) }
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
