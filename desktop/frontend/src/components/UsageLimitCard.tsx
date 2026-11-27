import type { LimitRow } from '../usage'

type UsageLimitCardProps = { row: LimitRow; color: string }

export function UsageLimitCard({ row, color }: UsageLimitCardProps) {
  return (
    <li className="grid items-center gap-x-6 gap-y-3 rounded-[10px] border border-line p-4 grid-cols-[minmax(0,8rem)_minmax(0,1fr)]">
      <div className="flex min-w-0 flex-col gap-1">
        <span className="truncate text-body font-medium text-ink">{row.label}</span>
        <span className="flex items-baseline gap-2">
          <span className="text-[1.875rem] font-semibold leading-none tabular-nums text-ink">{row.left}%</span>
          <span className="text-small text-soft">left</span>
        </span>
      </div>
      <div className="relative h-5 rounded-md bg-line" role="meter" aria-label={row.label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={row.left}>
        <div className="absolute inset-y-0 left-0 rounded-md" style={{ width: `${row.left}%`, backgroundColor: color }} />
        {row.resetsIn && (
          <span className="absolute right-2 top-1/2 -translate-y-1/2 rounded-sm bg-raised/85 px-1.5 py-0.5 text-label tabular-nums text-soft" title={`Resets in ${row.resetsIn}`}>
            ↻ {row.resetsIn}
          </span>
        )}
      </div>
    </li>
  )
}
