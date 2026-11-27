import type { Issue, IssueType } from './types'

export type Field = 'author' | 'assignee' | 'label' | 'type' | 'section' | 'is'

export type Token = { field: Field; value: string; negate: boolean }
export type Word = { text: string; negate: boolean }
export type ParsedQuery = { words: Word[]; tokens: Token[] }

export const TYPES: IssueType[] = ['bug', 'feature', 'refactor', 'chore']

// A section's name as it is typed after section:, which has no room for a space.
const sectionSlug = (name: string) => name.toLowerCase().replace(/\s+/g, '-')

const TOKEN = /^(author|assignee|label|type|section|is):(.*)$/i

export function parseQuery(query: string): ParsedQuery {
  const parsed: ParsedQuery = { words: [], tokens: [] }
  for (const part of query.split(/\s+/).filter(Boolean)) {
    const negate = part.startsWith('-') && part.length > 1
    const body = negate ? part.slice(1) : part
    const match = body.startsWith('@') ? ['', 'author', body.slice(1)] : TOKEN.exec(body)
    if (!match) {
      parsed.words.push({ text: body.replace(/^#/, '').toLowerCase(), negate })
      continue
    }
    const value = match[2].toLowerCase()
    if (value) parsed.tokens.push({ field: match[1].toLowerCase() as Field, value, negate })
  }
  return parsed
}

export function isFiltering(query: string): boolean {
  const { words, tokens } = parseQuery(query)
  return words.length + tokens.length > 0
}

function tokenMatches(issue: Issue, { field, value }: Token): boolean {
  if (field === 'author') return issue.author.toLowerCase().startsWith(value)
  if (field === 'assignee') {
    if (value === 'none') return issue.assignees.length === 0
    return issue.assignees.some((a) => a.toLowerCase().startsWith(value))
  }
  if (field === 'label') return issue.labels.some((l) => l.toLowerCase().startsWith(value))
  if (field === 'type') return issue.type === value
  if (field === 'section') return sectionSlug(issue.section) === value
  return value === 'running' && issue.threadId !== ''
}

export function matchIssue(issue: Issue, parsed: ParsedQuery): boolean {
  const haystack = `${issue.number} ${issue.title.toLowerCase()}`
  return (
    parsed.words.every((w) => haystack.includes(w.text) !== w.negate) &&
    parsed.tokens.every((t) => tokenMatches(issue, t) !== t.negate)
  )
}

export function filterIssues(issues: Issue[], query: string): Issue[] {
  const parsed = parseQuery(query)
  return issues.filter((issue) => matchIssue(issue, parsed))
}

export type Facet = { value: string; count: number }

export function facet<T>(items: T[], pick: (item: T) => string[]): Facet[] {
  const counts = new Map<string, number>()
  for (const item of items) for (const value of new Set(pick(item))) counts.set(value, (counts.get(value) ?? 0) + 1)
  return [...counts].map(([value, count]) => ({ value, count })).sort((a, b) => b.count - a.count || a.value.localeCompare(b.value))
}
