import type { DigestFilter } from '../digestFeed'

const FILTERS: { value: DigestFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'new', label: 'New' },
  { value: 'saved', label: 'Saved' },
]

type DigestFiltersProps = { value: DigestFilter; counts: Record<DigestFilter, number>; onChange(value: DigestFilter): void }

export function DigestFilters({ value, counts, onChange }: DigestFiltersProps) {
  return (
    <div role="group" aria-label="Show" className="segments-track shrink overflow-x-auto">
      {FILTERS.map((filter) => (
        <button key={filter.value} aria-pressed={value === filter.value} onClick={() => onChange(filter.value)}>
          {filter.label}
          <span className="ml-1 text-soft tabular-nums">{counts[filter.value]}</span>
        </button>
      ))}
    </div>
  )
}
