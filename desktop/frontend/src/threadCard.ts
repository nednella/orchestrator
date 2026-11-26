import type { PR, Session } from './types.ts'

export type CardIcon = 'machine' | 'git-branch' | 'git-pull-request' | 'git-pull-request-draft' | 'git-merge' | 'git-pull-request-closed' | 'circle-dot' | 'claude'

// One line of the hover card: the model line carries the agent's own model and effort ids, which the card names.
export type CardLine = { key: 'machine' | 'branch' | 'pr' | 'issue' | 'model'; icon: CardIcon; color?: string; text: string; detail?: string }

const BRANCH_MAX = 32

const PR_LOOK: Record<PR['state'], { icon: CardIcon; color: string; word: string }> = {
  open: { icon: 'git-pull-request', color: 'var(--pr-open)', word: 'Open' },
  draft: { icon: 'git-pull-request-draft', color: 'var(--pr-draft)', word: 'Draft' },
  merged: { icon: 'git-merge', color: 'var(--pr-merged)', word: 'Merged' },
  closed: { icon: 'git-pull-request-closed', color: 'var(--pr-closed)', word: 'Closed' },
}

// middleTruncate keeps the start and the end of text and puts … between them when it is longer than max characters.
export function middleTruncate(text: string, max: number): string {
  const chars = Array.from(text)
  if (chars.length <= max) return text
  const keep = Math.max(max - 1, 2)
  const head = Math.ceil(keep / 2)
  return `${chars.slice(0, head).join('')}…${chars.slice(chars.length - (keep - head)).join('')}`
}

// threadCardLines is what the hover card shows besides the title, in this order and each only with a value.
export function threadCardLines(s: Session, machine: string, issueTitle: string): CardLine[] {
  const lines: CardLine[] = []
  if (machine) lines.push({ key: 'machine', icon: 'machine', text: machine })
  if (s.branch) lines.push({ key: 'branch', icon: 'git-branch', text: middleTruncate(s.branch, BRANCH_MAX) })
  if (s.pr) {
    const look = PR_LOOK[s.pr.state]
    lines.push({ key: 'pr', icon: look.icon, color: look.color, text: s.pr.title ? `#${s.pr.number} ${s.pr.title}` : `#${s.pr.number}`, detail: look.word })
  }
  if (s.issue > 0) lines.push({ key: 'issue', icon: 'circle-dot', color: 'var(--pr-open)', text: issueTitle ? `#${s.issue} ${issueTitle}` : `#${s.issue}` })
  if (s.model || s.effort) lines.push({ key: 'model', icon: 'claude', color: 'var(--agent-claude)', text: s.model, detail: s.effort })
  return lines
}
