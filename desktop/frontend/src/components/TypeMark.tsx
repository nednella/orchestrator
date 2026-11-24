import type { IssueType } from '../types'

type TypeMarkProps = { type: IssueType; named?: boolean }

const COLOR: Record<IssueType, string> = {
  feature: 'var(--type-feature)',
  bug: 'var(--type-bug)',
  refactor: 'var(--type-refactor)',
  chore: 'var(--type-chore)',
  '': 'var(--text-dim)',
}

export function TypeMark({ type, named = false }: TypeMarkProps) {
  const mark = (
    <svg className="flex-none" width="12" height="12" viewBox="0 0 12 12" style={{ color: COLOR[type] }} aria-label={type || 'untyped'} role="img">
      <TypeShape type={type} />
    </svg>
  )
  if (!named || !type) return mark
  return (
    <span className="flex flex-none items-center gap-1.5" style={{ color: COLOR[type] }}>
      {mark}
      {type}
    </span>
  )
}

function TypeShape({ type }: TypeMarkProps) {
  if (type === 'feature') return <path d="M6 1 11 6 6 11 1 6Z" fill="currentColor" />
  if (type === 'bug') return <path d="M6 1.5 11 10.5H1Z" fill="currentColor" />
  if (type === 'refactor') return <circle cx="6" cy="6" r="4" fill="none" stroke="currentColor" strokeWidth="2" />
  if (type === 'chore') return <rect x="2" y="2" width="8" height="8" fill="currentColor" />
  return <path d="M2 6h8" stroke="currentColor" strokeWidth="2" />
}
