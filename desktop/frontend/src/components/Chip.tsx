import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

type ChipTone = 'ok' | 'warn' | 'bad' | 'dim'

const COLOR: Record<ChipTone, string> = {
  ok: 'var(--finished)',
  warn: 'var(--idle)',
  bad: 'var(--danger)',
  dim: 'var(--text-dim)',
}

type ChipProps = {
  tone: ChipTone
  icon: LucideIcon
  title?: string
  children?: ReactNode
}

export function Chip({ tone, icon: Glyph, title, children }: ChipProps) {
  return (
    <span
      className="inline-flex h-6 flex-none items-center gap-1.5 rounded-full border border-(--border-rule) bg-(--bg-card) pr-2.5 pl-2 text-label font-medium whitespace-nowrap text-soft tabular-nums"
      title={title}
    >
      <span className="flex" style={{ color: COLOR[tone] }}>
        <Glyph size={12} aria-hidden="true" />
      </span>
      {children}
    </span>
  )
}
