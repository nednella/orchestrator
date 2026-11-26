import type { AgentItem } from './chatTimeline'
import type { PR, Server, Session } from './types'

export const CARD_WIDTH = 280
export const CARD_GAP = 12
export const CHAT_LANE_PAD = 16
const CARD_CLEARANCE = 32
// CARD_ROOM is the width the card takes from the chat beside it: the card, its gap to the edge and its clearance.
export const CARD_ROOM = CARD_WIDTH + CARD_GAP + CARD_CLEARANCE
const MIN_CHAT_WIDTH = 640
const MIN_HEIGHT = 160

// CardFacts is what the thread's context card knows beside the thread itself. folder is the full path the thread
// works in; the parent is the thread it was delegated from or forked from, and forks are the threads forked from it.
export type CardFacts = {
  machine: string
  folder: string
  editor: string
  issueTitle: string
  servers: Server[]
  agents: AgentItem[]
  children: Session[]
  parent: Session | null
  forks: Session[]
}

export type CardRow =
  | { kind: 'workspace'; key: string; machine: string; folder: string; worktree: string }
  | { kind: 'editor'; key: string; editor: string }
  | { kind: 'branch'; key: string; branch: string }
  | { kind: 'pr'; key: string; pr: PR }
  | { kind: 'issue'; key: string; number: number; title: string }
  | { kind: 'server'; key: string; server: Server }
  | { kind: 'thread'; key: string; relation: 'parent' | 'child' | 'fork'; thread: Session }
  | { kind: 'agent'; key: string; agent: AgentItem }

export type CardSection = { key: 'workspace' | 'git' | 'servers' | 'lineage'; heading: string; rows: CardRow[] }

const present = <T>(row: T | false): row is T => row !== false

// cardSections are the card's sections in order, each with only the rows that have data, and only the sections that
// have a row: a fact the app does not know yet shows nothing.
export function cardSections(s: Session, f: CardFacts): CardSection[] {
  const workspace: (CardRow | false)[] = [
    (f.machine !== '' || f.folder !== '') && { kind: 'workspace', key: 'workspace', machine: f.machine, folder: f.folder, worktree: s.worktree },
    f.editor !== '' && { kind: 'editor', key: 'editor', editor: f.editor },
  ]
  const git: (CardRow | false)[] = [
    s.branch !== '' && { kind: 'branch', key: 'branch', branch: s.branch },
    s.pr !== null && { kind: 'pr', key: 'pr', pr: s.pr },
    s.issue > 0 && { kind: 'issue', key: 'issue', number: s.issue, title: f.issueTitle },
  ]
  const thread = (relation: 'parent' | 'child' | 'fork', t: Session): CardRow => ({ kind: 'thread', key: `${relation}-${t.id}`, relation, thread: t })
  const lineage: CardRow[] = [
    ...(f.parent ? [thread('parent', f.parent)] : []),
    ...f.children.map((c) => thread('child', c)),
    ...f.agents.map((a): CardRow => ({ kind: 'agent', key: `agent-${a.id}`, agent: a })),
    ...f.forks.map((fork) => thread('fork', fork)),
  ]
  const sections: CardSection[] = [
    { key: 'workspace', heading: '', rows: workspace.filter(present) },
    { key: 'git', heading: '', rows: git.filter(present) },
    { key: 'servers', heading: 'Servers', rows: f.servers.map((server) => ({ kind: 'server', key: `server-${server.port}`, server })) },
    { key: 'lineage', heading: lineageLabel(f), rows: lineage },
  ]
  return sections.filter((section) => section.rows.length > 0)
}

// lineageLabel heads the lineage: how many of the thread's delegated children and subagents work now, if any.
export function lineageLabel(f: CardFacts): string {
  const running = f.children.filter((c) => c.state === 'working').length + f.agents.filter((a) => a.state === 'running').length
  return running > 0 ? `Lineage · ${running} running` : 'Lineage'
}

// cardPlacement floats the card over the chat while the chat keeps room to read beside it, and else leaves it to a
// popover under its top-bar button.
export function cardPlacement(chatWidth: number, height: number): 'inline' | 'popover' {
  const left = chatWidth - CARD_ROOM - 2 * CHAT_LANE_PAD
  return left >= MIN_CHAT_WIDTH && height >= MIN_HEIGHT ? 'inline' : 'popover'
}

// folderName is the last part of a path.
export function folderName(path: string): string {
  return path.replace(/\/+$/, '').split('/').pop() ?? path
}

// serverProcess is the program a server runs, the first word of its command.
export function serverProcess(command: string): string {
  return folderName(command.trim().split(/\s+/)[0] ?? '')
}
