import { facet, filterIssues, TYPES } from './issueFilter.ts'
import type { Issue } from './types'

export type IssueSort = 'updated' | 'created'
export type IssueInvolvement = 'all' | 'assigned' | 'opened'

// IssuePrefs is all a view of the issue list holds: the search text, the sort, the filters and the provider.
export type IssuePrefs = {
  q: string
  sort: IssueSort
  involvement: IssueInvolvement
  type: string
  author: string
  assignee: string
  labels: string[]
  host: string
}

export const DEFAULT_ISSUE_PREFS: IssuePrefs = { q: '', sort: 'updated', involvement: 'all', type: '', author: '', assignee: '', labels: [], host: '' }

export const fixIssuePrefs = (value: unknown): IssuePrefs => {
  const stored = (value ?? {}) as Partial<IssuePrefs>
  return {
    q: typeof stored.q === 'string' ? stored.q : '',
    sort: stored.sort === 'created' ? 'created' : 'updated',
    involvement: stored.involvement === 'assigned' || stored.involvement === 'opened' ? stored.involvement : 'all',
    type: typeof stored.type === 'string' ? stored.type : '',
    author: typeof stored.author === 'string' ? stored.author : '',
    assignee: typeof stored.assignee === 'string' ? stored.assignee : '',
    labels: Array.isArray(stored.labels) ? stored.labels.filter((l) => typeof l === 'string') : [],
    host: typeof stored.host === 'string' ? stored.host : '',
  }
}

export const normalIssuePrefs = (p: IssuePrefs) => ({ ...p, q: p.q.trim(), labels: [...p.labels].sort() })

// issueFilterCount is the number on the Filters button: each filter that is set, a label each.
export function issueFilterCount(p: IssuePrefs): number {
  return (p.involvement !== 'all' ? 1 : 0) + (p.type ? 1 : 0) + (p.author ? 1 : 0) + (p.assignee ? 1 : 0) + p.labels.length
}

// listIssues is the issues the list shows, in the order of the sections the core gave and the sort inside each.
export function listIssues(issues: Issue[], p: IssuePrefs, me: string): Issue[] {
  const mine = me.toLowerCase()
  const shown = filterIssues(issues, p.q).filter(
    (i) =>
      (p.involvement !== 'assigned' || i.assignees.some((a) => a.toLowerCase() === mine)) &&
      (p.involvement !== 'opened' || i.author.toLowerCase() === mine) &&
      (!p.type || i.type === p.type) &&
      (!p.author || i.author === p.author) &&
      (!p.assignee || i.assignees.includes(p.assignee)) &&
      p.labels.every((l) => i.labels.includes(l)),
  )
  const rank = new Map<string, number>()
  for (const i of shown) if (!rank.has(i.section)) rank.set(i.section, rank.size)
  const stamp = (i: Issue) => (p.sort === 'created' ? i.createdAt : i.updatedAt)
  return [...shown].sort((a, b) => rank.get(a.section)! - rank.get(b.section)! || stamp(b) - stamp(a))
}

export type IssueFacets = {
  types: { value: string; count: number }[]
  authors: { value: string; count: number }[]
  assignees: { value: string; count: number }[]
  labels: { value: string; count: number }[]
}

export function issueFacets(issues: Issue[]): IssueFacets {
  return {
    types: TYPES.map((value) => ({ value, count: issues.filter((i) => i.type === value).length })).filter((f) => f.count > 0),
    authors: facet(issues, (i) => [i.author]),
    assignees: facet(issues, (i) => i.assignees),
    labels: facet(issues, (i) => i.labels),
  }
}
