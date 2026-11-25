import type { LucideIcon } from 'lucide-react'
import type { Change } from '../statsFigures'
import { StatsChange } from './StatsChange'

type StatsFigureProps = { icon: LucideIcon; label: string; value: string; sub: string; change?: Change | null; period?: string; title?: string }

export function StatsFigure({ icon: FigureIcon, label, value, sub, change, period = '', title }: StatsFigureProps) {
  return (
    <div title={title} className="flex min-w-0 flex-col gap-1 rounded-xl bg-(--bg-card) px-3.5 py-3 shadow-[inset_0_0_0_1px_var(--border)]">
      <dt className="flex items-center gap-1.5 text-small text-soft">
        <FigureIcon size={14} className="flex-none text-dim" aria-hidden="true" />
        {label}
      </dt>
      <dd className="text-(length:--fs-display) leading-[1.3] font-semibold whitespace-nowrap tabular-nums">{value}</dd>
      <dd className="flex flex-wrap items-center gap-x-2 text-small text-soft">
        {change && <StatsChange change={change} period={period} />}
        <span>{sub}</span>
      </dd>
    </div>
  )
}
