import { useState } from 'react'
import { ChartColumn } from 'lucide-react'
import { dollars } from '../spend'
import { busiest, chartBars, weekly, type Metric } from '../statsChart'
import { aDay, costAxis, counted, workAxis, workTime } from '../statsFigures'
import { readStored, writeStored } from '../storage'
import type { DayFigures } from '../types'
import { SegmentedControl } from './SegmentedControl'
import { StatsBars } from './StatsBars'
import { StatsCard } from './StatsCard'

type StatsPerDayProps = { days: DayFigures[] }

type MetricText = { name: string; total(sum: number): string; tip(value: number): string; axis(value: number, top: number): string }

const METRIC_KEY = 'orchestrator.statsMetric'

const OPTIONS: { value: Metric; label: string }[] = [
  { value: 'prompts', label: 'Prompts' },
  { value: 'worked', label: 'Worked' },
  { value: 'cost', label: 'Cost' },
]

const METRICS: Record<Metric, MetricText> = {
  prompts: {
    name: 'Prompts',
    total: (n) => n.toLocaleString(),
    tip: (n) => counted(n, 'prompt', 'prompts'),
    axis: (n) => Math.round(n).toLocaleString(),
  },
  worked: { name: 'Worked time', total: workTime, tip: (ms) => `${workTime(ms)} worked`, axis: workAxis },
  cost: { name: 'Estimated cost', total: (v) => `~${dollars(v)}`, tip: (v) => `~${dollars(v)}`, axis: costAxis },
}

export function StatsPerDay({ days }: StatsPerDayProps) {
  const [metric, setMetric] = useState<Metric>(() => {
    const stored = readStored<Metric>(METRIC_KEY, 'cost')
    return Object.hasOwn(METRICS, stored) ? stored : 'cost'
  })
  const pick = (next: Metric) => {
    setMetric(next)
    writeStored(METRIC_KEY, next)
  }
  const text = METRICS[metric]
  const bars = chartBars(days, metric)
  const byWeek = weekly(days)
  const sum = bars.reduce((a, b) => a + b.value, 0)
  const most = busiest(bars, byWeek)
  const top = Math.max(0, ...bars.map((b) => b.value))
  const each = metric === 'prompts' ? aDay(sum, days.length) : text.total(sum / Math.max(days.length, 1))
  return (
    <StatsCard icon={ChartColumn} title={byWeek ? 'Per week' : 'Per day'} end={<SegmentedControl variant="track" label="Per day shows" options={OPTIONS} value={metric} onChange={pick} />}>
      <p className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <b className="text-(length:--fs-display) font-semibold tabular-nums">{text.total(sum)}</b>
        <span className="text-small text-soft">
          {each} a day{most && ` · ${most}`}
        </span>
      </p>
      <StatsBars
        bars={bars}
        label={`${text.name} ${byWeek ? 'per week' : 'per day'}`}
        caption={byWeek ? 'The hatched bar is this week so far.' : 'The hatched bar is today so far.'}
        tip={text.tip}
        axis={(v) => text.axis(v, top)}
      />
    </StatsCard>
  )
}
