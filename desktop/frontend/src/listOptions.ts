import type { ReactNode } from 'react'

export type ListOption = { value: string; label: string; icon?: ReactNode; count?: number; unavailable?: string }

// A radio row picks one value; a check row picks any number. Both are one row of the Filters menu.
type RowBase = { id: string; label: string; icon: ReactNode; options: ListOption[]; searchLabel?: string; emptyText?: string }
export type RadioRow = RowBase & { kind: 'radio'; value: string; onChange(value: string): void }
export type CheckRow = RowBase & { kind: 'check'; value: string[]; onChange(value: string[]): void }
export type FilterRow = RadioRow | CheckRow | { kind: 'separator'; id: string }

export type SortConfig = { value: string; options: ListOption[]; onChange(value: string): void }

// rowValue is the text a Filters row shows for what it holds.
export function rowValue(row: RadioRow | CheckRow): string {
  if (row.kind === 'radio') return row.options.find((o) => o.value === row.value)?.label ?? 'Any'
  if (row.value.length === 0) return 'Any'
  if (row.value.length === 1) return row.options.find((o) => o.value === row.value[0])?.label ?? row.value[0]
  return `${row.value.length} selected`
}

// shownOptions are the options that match the search text, the picked ones first so they can be cleared.
export function shownOptions(row: RadioRow | CheckRow, search: string, limit = 30): ListOption[] {
  const needle = search.trim().toLowerCase()
  const picked = row.kind === 'check' ? row.value : []
  const matches = row.options.filter((o) => o.label.toLowerCase().includes(needle))
  return [...matches.filter((o) => picked.includes(o.value)), ...matches.filter((o) => !picked.includes(o.value))].slice(0, limit)
}

export function toggled(values: string[], value: string): string[] {
  return values.includes(value) ? values.filter((v) => v !== value) : [...values, value]
}

// facetOptions are the options a facet gives, and the values picked that it no longer has, so they can be cleared.
export function facetOptions(facets: { value: string; count: number }[], picked: string[]): ListOption[] {
  const gone = picked.filter((p) => !facets.some((f) => f.value === p)).map((value) => ({ value, label: value }))
  return [...gone, ...facets.map((f) => ({ value: f.value, label: f.value, count: f.count }))]
}
