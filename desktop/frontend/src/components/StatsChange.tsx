import { ArrowDown, ArrowUp } from 'lucide-react'
import { changeLabel, type Change } from '../statsFigures'

type StatsChangeProps = { change: Change; period: string }

export function StatsChange({ change, period }: StatsChangeProps) {
  const label = changeLabel(change, period)
  return (
    <span role="img" aria-label={label} title={label} className="inline-flex items-center gap-0.5 text-ink tabular-nums">
      {change.direction === 'up' && <ArrowUp size={12} aria-hidden="true" />}
      {change.direction === 'down' && <ArrowDown size={12} aria-hidden="true" />}
      {change.text}
    </span>
  )
}
