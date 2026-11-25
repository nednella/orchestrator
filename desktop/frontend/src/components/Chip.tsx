import type { ReactNode } from 'react'
import { Icon } from './Icon'
import type { IconName } from './Icon'

type ChipTone = 'ok' | 'warn' | 'bad' | 'dim'

const COLOR: Record<ChipTone, string> = {
  ok: 'var(--finished)',
  warn: 'var(--idle)',
  bad: 'var(--danger)',
  dim: 'var(--text-dim)',
}

type ChipProps = {
  tone: ChipTone
  icon: IconName
  title?: string
  children?: ReactNode
}

export function Chip({ tone, icon, title, children }: ChipProps) {
  return (
    <span
      className="inline-flex h-5 flex-none items-center gap-1 rounded-sm px-1.5 text-label font-medium whitespace-nowrap"
      style={{ color: COLOR[tone], background: 'color-mix(in srgb, currentColor 14%, transparent)' }}
      title={title}
    >
      <Icon name={icon} size={12} />
      {children}
    </span>
  )
}
