import type { Access, ChatEvent, Pending } from './types'

// Command is one slash command, as Claude lists them in a commands event.
export type Command = { name: string; description: string; argumentHint: string; builtin?: boolean; aliases?: string[] }

// A step is one thing a fold holds, in the order it came: a tool call, text Claude wrote between calls, or a thinking
// block, live while it streams.
export type WorkStep = { kind: 'call'; call: ToolCall } | { kind: 'note'; key: string; text: string } | { kind: 'thought'; key: string; text: string; live: boolean }

export type ToolCall = { id: string; tool: string; input: unknown; result: string | null; error: boolean; answer?: 'allow' | 'deny' }

// AgentState is where a subagent is: at work, done, failed, or stopped, by the owner or by its Claude's exit.
export type AgentState = 'running' | 'done' | 'failed' | 'stopped'

// AgentItem is an Agent call of the thread's own Claude and what its subagent did: its tool calls, thinking and text,
// and its report once it ends. turn is the key of the turn that made the call; type is the subagent's type, '' for the
// default one.
export type AgentItem = { kind: 'agent'; key: string; id: string; turn: string; description: string; type: string; startedAt: number; endedAt: number | null; state: AgentState; tools: number; steps: WorkStep[]; result: string | null }

// Ask is a question Claude puts to the user, or a plan it asks the user to approve. request is the approval that
// answers it, '' until Claude asks for one. answers is what a question was answered with, by question, once known.
export type Ask = { key: string; at: number; id: string; input: unknown; request: string; answer?: 'allow' | 'deny'; answers: Record<string, string> | null; expired: boolean; result: string | null }

// ChildItem is a delegate call of the thread's Claude: the child thread it started, by the id the call's result gives,
// '' until the result comes. A failed call shows as a tool row; error marks one that failed after its turn ended, and
// expired one that never got its result: Claude's turn ended, or Claude stopped, while the call ran.
export type ChildItem = { kind: 'child'; key: string; at: number; call: string; child: string; title: string; error: boolean; expired: boolean }

export type TimelineItem =
  // forkAt is the uuid of the reply before the message, which a fork from the message continues from; '' when none or
  // when the reply is from before the last compaction. uuid is the message's own in Claude's conversation, which an
  // edit from the message rewinds to; '' for the text of a denial, which Claude holds as no message, and for a message
  // from before the last compaction. from is the thread that sent it, missing for the owner's.
  | { kind: 'user'; key: string; at: number; id: string; uuid: string; text: string; files: string[]; forkAt: string; from?: string }
  // childResult is a message from a child of the thread: its result.
  | { kind: 'childResult'; key: string; at: number; child: string; text: string }
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
  | AgentItem
  | ChildItem

// TurnEnd is how a turn ended that did not finish: stopped by the owner or Claude's exit, or failed.
export type TurnEnd = 'stopped' | 'failed'

export type QueuedMessage = { id: string; text: string; files: string[] }

// agentsRunning is whether a subagent still works, which it does in the background after the turn that started it.
export type Timeline = { items: TimelineItem[]; queued: QueuedMessage[]; turnOpen: boolean; agentsRunning: boolean; pending: Pending | null; commands: Command[] }

type Notice = Extract<TimelineItem, { kind: 'notice' }>
type Restart = Extract<TimelineItem, { kind: 'restart' }>
type ApprovalItem = Extract<TimelineItem, { kind: 'approval' }>
type AskItem = Extract<TimelineItem, { kind: 'question' | 'plan' }>
type Reply = { kind: 'reply'; key: string; at: number; id: string; uuid: string; blocks: string[] }
type ReplyItem = Extract<TimelineItem, { kind: 'reply' }>
type Thought = { kind: 'thought'; key: string; id: string; text: string }
// A part is one thing a turn holds, in the order it came.
type Part = Reply | Thought | { kind: 'call'; call: ToolCall } | AskItem | ApprovalItem | AgentItem | ChildItem
// A delegation is a delegate call: its card, the call it turns back into if it fails, and the turn that made it.
type Delegation = { item: ChildItem; call: ToolCall; turn: Turn }
// An approval answers an ask, or else shows as an item of its own, next to the call it is for when there is one. agent
// is the Agent call of the subagent that asks, whose row the item shows after.
type Approval = { request: string; tool: string; ask?: AskItem; call?: ToolCall; item?: ApprovalItem; answered: boolean; agent?: string }
// unsent holds the not-sent notices logged during the turn: the host drops a message before the turn ends, and the
// notice shows after the turn's own.
type Turn = { key: string; startedAt: number; parts: Part[]; approvals: Approval[]; unsent: Notice[] }

// One empty list for every message without images keeps a memoised row from rendering again.
const noFiles: string[] = []

// turnError is why Claude says a turn failed, without the diagnostic lines it adds for its own use; '' when none.
const turnError = (errors = '') => errors.split('\n').filter((line) => !line.startsWith('[ede_diagnostic]')).join('\n').trim()

const askKinds: Record<string, 'question' | 'plan'> = { AskUserQuestion: 'question', ExitPlanMode: 'plan' }

// Claude starts a subagent with its Agent tool, which older versions call Task.
const agentTools = new Set(['Agent', 'Task'])
// A subagent of the general type is not named by its type.
const defaultAgentType = 'general-purpose'

// delegateTool is the agentos tool that starts a child thread.
const delegateTool = 'mcp__agentos__delegate'
// The app starts each text it writes to Claude with this, so Claude does not take it for the owner's.
const appPrompt = 'APPLICATION_PROMPT: '

const fromApp = (text: string) => (text.startsWith(appPrompt) ? text.slice(appPrompt.length) : text)

const childTitle = (input: unknown) => {
  const { title, task } = (input ?? {}) as { title?: unknown; task?: unknown }
  if (typeof title === 'string' && title.trim() !== '') return title.trim()
  return (typeof task === 'string' && firstLine(task)) || 'Child thread'
}

// childOf is the id of the child a delegate call started, from its result; '' when the result holds none.
const childOf = (result = '') => {
  try {
    const id = (JSON.parse(result) as { id?: unknown } | null)?.id
    return typeof id === 'string' ? id : ''
  } catch {
    return ''
  }
}

const agentOf = (input: unknown) => {
  const { description, subagent_type: type } = (input ?? {}) as { description?: unknown; subagent_type?: unknown }
  return { description: typeof description === 'string' && description !== '' ? description : 'Subagent', type: typeof type === 'string' && type !== defaultAgentType ? type : '' }
}

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
  const agents = new Map<string, AgentItem>()
  const delegations = new Map<string, Delegation>()
  const children = new Set<string>()
  // The calls each subagent makes, by call, to the Agent call of the subagent.
  const agentOfCall = new Map<string, string>()
  // The approvals subagents ask for, oldest first: a subagent in the background asks after the turn that started it
  // ended, and its approval waits until it is answered, its call has its result, the subagent ends or its Claude goes.
  const agentApprovals: Approval[] = []
  let commands: Command[] = []
  let turn: Turn | null = null
  let interrupted = false
  // The tasks other than shells that Claude runs in the background, by the call that started each. While one runs, or
  // until Claude has told of the end of the last one in a turn of its own, the turns are one piece of work: the turn
  // rests at its end, from held, and Claude's next turn goes on with it. A shell may be a server that never ends.
  const tasks = new Set<string>()
  let held: number | null = null
  // Claude resumes a compacted conversation from its last compaction and cannot find a message before it, so no fork
  // ends at one.
  const compacted = events.findLastIndex((e) => e.kind === 'compact')

  const open = (e: ChatEvent): Turn => {
    held = null
    return (turn ??= { key: (e.kind === 'user' && e.id) || `seq-${e.seq}`, startedAt: e.at, parts: [], approvals: [], unsent: [] })
  }

  const unsent = (id: string, text: string, at: number, error = false): Notice => {
    queued.delete(id)
    return { kind: 'notice', key: `unsent-${id}`, at, text: `Not sent: ${firstLine(text)}`, error }
  }

  // close folds the turn into the items, then its notice and its not-sent notices, and gives back the approvals it
  // left unanswered.
  const close = (endedAt: number, notice: Notice | Restart | null = null, ended?: TurnEnd): Approval[] => {
    const t = turn as Turn | null
    turn = null
    held = null
    if (t) items.push(...folded(t, endedAt, ended))
    if (notice) items.push(notice)
    if (t) items.push(...t.unsent)
    return t?.approvals.filter((a) => !a.answered) ?? []
  }

  // expire marks what the turn left unanswered, and each delegate call that has no result: it can no longer be answered.
  const expire = () => {
    for (const a of turn?.approvals ?? []) {
      if (a.answered) continue
      if (a.ask) a.ask.expired = true
      if (a.item) a.item.expired = true
    }
    for (const d of delegations.values()) if (d.call.result === null) d.item.expired = true
  }

  // end closes the turn for good.
  const end = (e: ChatEvent, notice: Notice | Restart | null, ended?: TurnEnd) => {
    expire()
    close(e.at, notice, ended)
  }

  // stopAgents stops each subagent still at work: its Claude is gone.
  const stopAgents = (at: number) => {
    for (const a of agents.values()) if (a.state === 'running') Object.assign(a, { state: 'stopped', endedAt: at })
    expireAgentApprovals(() => true)
  }

  // expireAgentApprovals marks each subagent's approval that waits and matches as one that can no longer be answered.
  const expireAgentApprovals = (match: (a: Approval) => boolean) => {
    for (const a of agentApprovals) if (!a.answered && a.item && match(a)) a.item.expired = true
  }

  // subagent adds what a subagent did to its Agent call's item, never to the turn: a subagent in the background goes on
  // after the turn that started it ends. A subagent of a subagent shows as its parent's call.
  const subagent = (e: ChatEvent, a: AgentItem) => {
    switch (e.kind) {
      case 'message':
        a.steps.push({ kind: 'note', key: `note-${e.seq}`, text: e.text ?? '' })
        return
      case 'thought':
        a.steps.push({ kind: 'thought', key: `thought-${e.seq}`, text: e.text ?? '', live: false })
        return
      case 'tool_use': {
        const call: ToolCall = { id: e.id || `seq-${e.seq}`, tool: e.tool ?? '', input: e.input, result: null, error: false }
        calls.set(call.id, call)
        agentOfCall.set(call.id, a.id)
        a.steps.push({ kind: 'call', call })
        a.tools++
        return
      }
    }
  }

  // delegated takes a delegate call's result: the child it started, or its error, which turns the card back into the
  // call's tool row while its turn runs. A result that comes after its turn ended is the call's answer all the same.
  const delegated = (d: Delegation, e: ChatEvent) => {
    d.call.result = e.text ?? ''
    d.item.expired = false
    if (!e.error) {
      d.item.child = childOf(e.text)
      if (d.item.child) children.add(d.item.child)
      return
    }
    d.call.error = true
    const at = turn === d.turn ? d.turn.parts.indexOf(d.item) : -1
    if (at >= 0) d.turn.parts[at] = { kind: 'call', call: d.call }
    else d.item.error = true
  }

  // agentApproval keeps a subagent's approval for its call, apart from the turns.
  const agentApproval = (e: ChatEvent, agent: string) => {
    const a: Approval = { request: e.id ?? '', tool: e.tool ?? '', call: calls.get(e.parent ?? ''), answered: false, agent }
    a.item = { kind: 'approval', key: `approval-${a.request}`, at: e.at, request: a.request, tool: a.tool, input: e.input, expired: false }
    approvals.set(a.request, a)
    agentApprovals.push(a)
  }

  events.forEach((e, i) => {
    // A log from before the host named the subagent that asks names only the call.
    const asker = e.kind === 'approval' ? e.agent || agentOfCall.get(e.parent ?? '') : undefined
    if (asker) {
      if (agents.has(asker)) agentApproval(e, asker)
      return
    }
    // A subagent's tool result finds its call as any other does, and its tasks are the thread's.
    if (e.agent && e.kind !== 'tool_result' && e.kind !== 'task_start' && e.kind !== 'task_end') {
      const a = agents.get(e.agent)
      if (a) subagent(e, a)
      return
    }
    switch (e.kind) {
      case 'user': {
        // A message Claude takes in while it works splits the turn there.
        const left = close(held ?? e.at)
        if (e.id) queued.delete(e.id)
        const forkAt = items.findLast((i): i is ReplyItem => i.kind === 'reply')?.uuid ?? ''
        const uuid = e.parent || i <= compacted ? '' : (e.id ?? '')
        const from = e.from ?? ''
        const text = from ? fromApp(e.text ?? '') : (e.text ?? '')
        if (children.has(from)) items.push({ kind: 'childResult', key: `result-${e.id || e.seq}`, at: e.at, child: from, text })
        else items.push({ kind: 'user', key: `user-${e.id || e.seq}`, at: e.at, id: e.id ?? '', uuid, text, files: e.files ?? noFiles, forkAt, ...(from ? { from } : {}) })
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
        if (agentTools.has(e.tool ?? '')) {
          const a: AgentItem = { kind: 'agent', key: `agent-${id}`, id, turn: t.key, ...agentOf(e.input), startedAt: e.at, endedAt: null, state: 'running', tools: 0, steps: [], result: null }
          agents.set(id, a)
          t.parts.push(a)
          return
        }
        if (e.tool === delegateTool) {
          const item: ChildItem = { kind: 'child', key: `child-${id}`, at: e.at, call: id, child: '', title: childTitle(e.input), error: false, expired: false }
          delegations.set(id, { item, call: { id, tool: delegateTool, input: e.input, result: null, error: false }, turn: t })
          t.parts.push(item)
          return
        }
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
        const d = delegations.get(e.parent ?? '')
        if (d) {
          delegated(d, e)
          return
        }
        // An Agent call's own result says only that its subagent started, or holds its report in a frame: the
        // subagent's end says the rest. A failed call started none.
        const a = agents.get(e.parent ?? '')
        if (a?.state === 'running' && e.error) Object.assign(a, { state: 'failed', endedAt: e.at, result: e.text ?? '' })
        expireAgentApprovals((w) => w.call?.id === e.parent || w.agent === e.parent)
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
        if (turn && a.item && !a.agent) turn.parts = turn.parts.filter((p) => p !== a.item)
        return
      }
      case 'task_start': {
        if (e.tool !== 'local_bash') tasks.add(e.parent ?? '')
        // Claude wakes a subagent that ended again under its call.
        const a = agents.get(e.parent ?? '')
        if (a && a.state !== 'running') Object.assign(a, { state: 'running', endedAt: null, result: null })
        return
      }
      case 'task_end': {
        // Claude tells of the end of a task of its own that was not stopped in a turn it starts itself.
        if (tasks.delete(e.parent ?? '') && held !== null && tasks.size === 0 && (e.agent || e.stop === 'stopped')) close(held)
        expireAgentApprovals((w) => w.agent === e.parent)
        const a = agents.get(e.parent ?? '')
        if (a?.state !== 'running') return
        Object.assign(a, { state: e.stop === 'stopped' ? 'stopped' : e.error ? 'failed' : 'done', endedAt: e.at, result: e.text ?? '' })
        return
      }
      case 'interrupt':
        if (turn) interrupted = true
        return
      case 'turn_end':
        if (!e.error && turn && tasks.size > 0) {
          expire()
          held = e.at
          return
        }
        end(e, e.error ? noticeOf(e, interrupted ? 'Stopped' : turnError(e.text) || 'The turn ended with an error', !interrupted) : null, e.error ? (interrupted ? 'stopped' : 'failed') : undefined)
        interrupted = false
        return
      case 'exit':
        interrupted = false
        tasks.clear()
        stopAgents(e.at)
        end(e, exitItem(e), e.error ? 'failed' : 'stopped')
        // Claude is gone, so a queued message it did not take in will not be sent.
        for (const q of queued.values()) items.push(unsent(q.id, q.text, e.at))
        return
      // A run whose host was killed logged no exit: its turn and what it waits on end where the next run starts. A start
      // with text is the same run's Claude started again without a flag it did not know, before it read anything.
      case 'start':
        if (e.text) return
        interrupted = false
        tasks.clear()
        stopAgents(e.at)
        end(e, null, 'stopped')
        return
      case 'commands':
        commands = Array.isArray(e.input) ? e.input.filter((c): c is Command => typeof c?.name === 'string') : []
        return
      // Claude's conversation went back to before the message: it and everything after it are gone from the chat. A
      // rewind waits for the turn's end, so no turn is open.
      case 'rewind': {
        const at = items.findIndex((item) => item.kind === 'user' && item.uuid !== '' && item.id === e.id)
        if (at >= 0) items.length = at
        return
      }
    }
  })

  // A subagent's report is most often its last text, which then shows once.
  for (const a of agents.values()) {
    const last = a.steps.at(-1)
    if (a.result && last?.kind === 'note' && last.text.trim() === a.result.trim()) a.steps.pop()
  }

  const running = turn as Turn | null
  if (running) items.push(...unfolded(running, live, thinking, events.at(-1)?.at ?? 0))
  // A subagent's approval shows after its row until it is answered.
  for (const a of agentApprovals.toReversed()) {
    const at = items.findIndex((item) => item.kind === 'agent' && item.id === a.agent)
    if (!a.answered && a.item && at >= 0) items.splice(at + 1, 0, a.item)
  }
  const unanswered = (a: Approval) => !a.answered && !a.ask?.expired && !a.item?.expired
  const waiting = running?.approvals.find(unanswered) ?? agentApprovals.find(unanswered)
  return {
    items,
    queued: [...queued.values()],
    turnOpen: running !== null && held === null,
    agentsRunning: [...agents.values()].some((a) => a.state === 'running'),
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
  return noticeOf(e, e.stop === 'settle' ? 'Asleep' : 'Claude stopped', e.error ?? false)
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
  if (!turn.parts.some((p) => p.kind === 'call' || p.kind === 'thought')) return [work([]), ...turn.parts.map((p) => (p.kind === 'reply' ? reply(p) : (p as AskItem | ApprovalItem | AgentItem | ChildItem)))]
  const replies = turn.parts.filter((p): p is Reply => p.kind === 'reply')
  const rest = turn.parts.filter((p): p is AskItem | ApprovalItem | AgentItem | ChildItem => p.kind !== 'call' && p.kind !== 'reply' && p.kind !== 'thought')
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
  const raw = a.ask?.input ?? a.item?.input ?? null
  const input = (raw ?? {}) as { command?: string; file_path?: string; path?: string; pattern?: string; url?: string; plan?: string; questions?: { question?: string }[] }
  const kind = approvalKind(a.tool)
  let summary = ''
  if (kind === 'question') summary = input.questions?.[0]?.question ?? ''
  else if (kind === 'plan') summary = firstLine(input.plan ?? '')
  else summary = firstLine([input.command, input.file_path, input.path, input.pattern, input.url].find((s) => typeof s === 'string' && s !== '') ?? '')
  return { id: thread, request: a.request, tool: a.tool, summary, kind, input: raw, inputTooLarge: false }
}

// sameSteps is whether two builds of the timeline hold the same steps: it is built anew on every streamed delta, so its
// steps are new objects holding the same values.
export function sameSteps(a: WorkStep[], b: WorkStep[]): boolean {
  return a.length === b.length && a.every((s, i) => sameStep(s, b[i]))
}

function sameStep(a: WorkStep, b: WorkStep): boolean {
  if (a.kind === 'note') return b.kind === 'note' && a.text === b.text
  if (a.kind === 'thought') return b.kind === 'thought' && a.key === b.key && a.text === b.text && a.live === b.live
  return b.kind === 'call' && a.call.id === b.call.id && a.call.tool === b.call.tool && a.call.input === b.call.input && a.call.result === b.call.result && a.call.error === b.call.error && a.call.answer === b.call.answer
}

// agentList is the thread's subagents for its list, the newest turn's first and each turn's in the order Claude
// started them.
export function agentList(items: TimelineItem[]): AgentItem[] {
  const turns = new Map<string, AgentItem[]>()
  for (const item of items) if (item.kind === 'agent') turns.set(item.turn, [...(turns.get(item.turn) ?? []), item])
  return [...turns.values()].reverse().flat()
}

export function firstLine(s: string): string {
  return (
    s
      .split('\n')
      .map((line) => line.trim())
      .find((line) => line !== '') ?? ''
  )
}
