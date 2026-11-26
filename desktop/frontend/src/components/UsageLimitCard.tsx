import type { LimitRow } from '../usage'

const FILL = { plain: 'bg-[var(--text-soft)]', warning: 'bg-idle', danger: 'bg-danger' }
const VALUE = { plain: 'text-ink', warning: 'text-idle', danger: 'text-danger' }

type UsageLimitCardProps = { row: LimitRow }

export function UsageLimitCard({ row }: UsageLimitCardProps) {
  return (
    <li className="flex flex-col gap-2 rounded-[10px] border border-line px-3 py-3">
      <div className="flex items-baseline justify-between gap-3">
        <span className="min-w-0 truncate text-body text-soft">{row.label}</span>
        <span className={`flex-none text-[1.25rem] font-semibold leading-none ${VALUE[row.tone]}`}>{row.left}% left</span>
      </div>
      <div className="h-[12px] overflow-hidden rounded-full bg-line" role="meter" aria-label={row.label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={row.left}>
        <div className={`h-full rounded-full ${FILL[row.tone]}`} style={{ width: `${row.left}%` }} />
      </div>
      {row.resetsIn && <span className="text-small text-dim">Resets in {row.resetsIn}</span>}
    </li>
  )
}
