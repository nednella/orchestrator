import { Bug, Minus, Recycle, Sparkles, Wrench } from 'lucide-react'
import type { IssueType } from '../types'

type TypeMarkProps = { type: IssueType; named?: boolean }

const COLOR: Record<IssueType, string> = {
  feature: 'var(--type-feature)',
  bug: 'var(--type-bug)',
  refactor: 'var(--type-refactor)',
  chore: 'var(--type-chore)',
  '': 'var(--text-dim)',
}

const ICON = { feature: Sparkles, bug: Bug, refactor: Recycle, chore: Wrench, '': Minus }

export function TypeMark({ type, named = false }: TypeMarkProps) {
  const Shape = ICON[type]
  const mark = <Shape className="flex-none" size={14} color={COLOR[type]} role="img" aria-label={type || 'untyped'} />
  if (!named || !type) return mark
  return (
    <span className="flex flex-none items-center gap-1.5" style={{ color: COLOR[type] }}>
      {mark}
      {type}
    </span>
  )
}
