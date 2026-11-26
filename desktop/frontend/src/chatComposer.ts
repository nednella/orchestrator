import type { Command, Timeline } from './chatTimeline'
import type { ChatEvent } from './types'

export type EscapeAction = 'close-overlay' | 'close-picker' | 'send-now' | 'interrupt' | 'blur' | 'none'
export type ComposerAction = 'send' | 'queue' | 'deny' | 'stop'

// escapeAction is what Esc does in a thread's chat: it closes what is open over the chat first, then stops a running
// turn only once, so that Claude takes in the queued messages at once or, with none queued, waits.
export function escapeAction({ overlay, picker, turnOpen, stopping, queued }: { overlay: boolean; picker: boolean; turnOpen: boolean; stopping: boolean; queued: boolean }): EscapeAction {
  if (overlay) return 'close-overlay'
  if (picker) return 'close-picker'
  if (!turnOpen) return 'blur'
  if (stopping) return 'none'
  return queued ? 'send-now' : 'interrupt'
}

// composerAction is what the composer's button does: with the turn open and nothing typed it stops the turn, and a
// message denies the approval that waits, or is queued while the turn runs.
export function composerAction({ turnOpen, pending }: Pick<Timeline, 'turnOpen' | 'pending'>, draftEmpty: boolean): ComposerAction {
  if (turnOpen && draftEmpty) return 'stop'
  if (pending) return 'deny'
  return turnOpen ? 'queue' : 'send'
}

// unseenDrafts is the drafts orchestrator left in the thread's composer that it has not shown yet: those after the seen
// mark and after the last message the user sent, which took in any draft before it.
export function unseenDrafts(events: ChatEvent[], seenSeq: number): ChatEvent[] {
  const after = Math.max(seenSeq, events.findLast((e) => e.kind === 'user')?.seq ?? 0)
  return events.filter((e) => e.kind === 'draft' && e.seq > after)
}

// sendText is the message the composer's markdown sends. A leading indent can be code, so only the end is trimmed.
export const sendText = (markdown: string) => markdown.replace(/\s+$/, '')

// lastTurnEnd is the seq of the latest event that ended a turn: a stop asked for after it waits for the next one.
export const lastTurnEnd = (events: ChatEvent[]) => events.findLast((e) => e.kind === 'turn_end' || e.kind === 'exit')?.seq ?? 0

const quoted = (text: string) => text.split('\n').map((line) => `> ${line}`).join('\n')

// quoteMarkdown is a selection as a markdown quote for the composer: nothing for an empty one.
export const quoteMarkdown = (text: string) => (text.trim() === '' ? '' : `${quoted(text.replace(/\s+$/, ''))}\n\n`)

const REPLY_LINES = 3
const REPLY_CHARS = 300

// replyPrefix is the start of a message that answers another: the first lines of that message, quoted.
export function replyPrefix(text: string): string {
  const head = text.trim().split('\n').slice(0, REPLY_LINES).join('\n')
  return quoteMarkdown(head.length > REPLY_CHARS ? `${head.slice(0, REPLY_CHARS - 1)}…` : head)
}

const LABEL_CHARS = 80

// replyLabel is what the reply chip shows of the message: its first line.
export function replyLabel(text: string): string {
  const line = text.trim().split('\n')[0]
  return line.length > LABEL_CHARS ? `${line.slice(0, LABEL_CHARS - 1)}…` : line
}

// mentionToken is what the @ mention the caret ends has typed after its @, or null when the caret ends none.
export const mentionToken = (before: string): string | null => /(^|\s)@([\w./-]*)$/.exec(before)?.[2] ?? null

// commandToken is what follows the / that starts the message, while the caret is still in that first word, or null.
export const commandToken = (before: string): string | null => /^\/([\w:.-]*)$/.exec(before)?.[1] ?? null

// commandMatches is the commands whose name starts with query, then those whose name has it elsewhere, ignoring case.
export function commandMatches(commands: Command[], query: string): Command[] {
  const q = query.toLowerCase()
  const starts = (c: Command) => c.name.toLowerCase().startsWith(q)
  return [...commands.filter(starts), ...commands.filter((c) => !starts(c) && c.name.toLowerCase().includes(q))]
}
