import { facet, parseQuery } from './issueFilter.ts'
import type { RepoPull } from './types'

export type PullSort = 'updated' | 'created' | 'oldest'
export type PullInvolvement = 'all' | 'reviewing' | 'authored'
export type PullDraft = 'all' | 'only' | 'hide'

export type PullPrefs = {
  q: string
  sort: PullSort
  involvement: PullInvolvement
  draft: PullDraft
  author: string
  labels: string[]
  host: string
}

export const DEFAULT_PULL_PREFS: PullPrefs = { q: '', sort: 'updated', involvement: 'all', draft: 'all', author: '', labels: [], host: '' }

export const fixPullPrefs = (value: unknown): PullPrefs => {
  const stored = (value ?? {}) as Partial<PullPrefs>
  return {
    q: typeof stored.q === 'string' ? stored.q : '',
    sort: stored.sort === 'created' || stored.sort === 'oldest' ? stored.sort : 'updated',
    involvement: stored.involvement === 'reviewing' || stored.involvement === 'authored' ? stored.involvement : 'all',
    draft: stored.draft === 'only' || stored.draft === 'hide' ? stored.draft : 'all',
    author: typeof stored.author === 'string' ? stored.author : '',
    labels: Array.isArray(stored.labels) ? stored.labels.filter((l) => typeof l === 'string') : [],
    host: typeof stored.host === 'string' ? stored.host : '',
  }
}

export const normalPullPrefs = (p: PullPrefs) => ({ ...p, q: p.q.trim(), labels: [...p.labels].sort() })

export function pullFilterCount(p: PullPrefs): number {
  return (p.involvement !== 'all' ? 1 : 0) + (p.draft !== 'all' ? 1 : 0) + (p.author ? 1 : 0) + p.labels.length
}

// The search text matches the number, title, branch and author; author: (or @) and label: narrow by those.
function matchesText(pull: RepoPull, q: string): boolean {
  const { words, tokens } = parseQuery(q)
  const haystack = `${pull.number} ${pull.title} ${pull.branch} ${pull.author}`.toLowerCase()
  return (
    words.every((w) => haystack.includes(w.text) !== w.negate) &&
    tokens.every((t) => {
      if (t.field === 'author') return pull.author.toLowerCase().startsWith(t.value) !== t.negate
      if (t.field === 'label') return pull.labels.some((l) => l.toLowerCase().startsWith(t.value)) !== t.negate
      return true
    })
  )
}

export function listPulls(pulls: RepoPull[], p: PullPrefs, me: string): RepoPull[] {
  const mine = me.toLowerCase()
  const shown = pulls.filter(
    (pull) =>
      matchesText(pull, p.q) &&
      (p.involvement !== 'reviewing' || pull.asked.some((a) => a.toLowerCase() === mine)) &&
      (p.involvement !== 'authored' || pull.author.toLowerCase() === mine) &&
      (p.draft !== 'only' || pull.draft) &&
      (p.draft !== 'hide' || !pull.draft) &&
      (!p.author || pull.author === p.author) &&
      p.labels.every((l) => pull.labels.includes(l)),
  )
  const order: Record<PullSort, (a: RepoPull, b: RepoPull) => number> = {
    updated: (a, b) => b.updatedAt - a.updatedAt,
    created: (a, b) => b.createdAt - a.createdAt,
    oldest: (a, b) => a.createdAt - b.createdAt,
  }
  return [...shown].sort(order[p.sort])
}

export function pullFacets(pulls: RepoPull[]) {
  return { authors: facet(pulls, (p) => [p.author]), labels: facet(pulls, (p) => p.labels) }
}
