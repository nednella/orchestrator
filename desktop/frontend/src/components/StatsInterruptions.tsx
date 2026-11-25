import { useEffect, useState } from 'react'
import { Hand, Hourglass, Timer } from 'lucide-react'
import { api, on } from '../api'
import { useAgentos } from '../AgentosContext'
import { aDay, kindCounts, workTime } from '../statsFigures'
import { KIND_COLOR, KIND_ICON, KIND_LABEL } from '../statsMeta'
import type { Stats } from '../types'
import { StatsCard } from './StatsCard'
import { StatsCause } from './StatsCause'

type StatsInterruptionsProps = { days: number; project: string }

const TOP_CAUSES = 5

export function StatsInterruptions({ days, project }: StatsInterruptionsProps) {
  const { report } = useAgentos('report')
  const [stats, setStats] = useState<Stats | null>(null)

  useEffect(() => {
    let current = true
    const load = () =>
      report(async () => {
        const next = await api.stats(days, project)
        if (current) setStats(next)
      })
    load()
    const off = on('stats', load)
    return () => {
      current = false
      off()
    }
  }, [days, project, report])

  if (stats === null) return null
  const kinds = kindCounts(stats.byCause)
  return (
    <StatsCard icon={Hand} title="Interruptions" meta="times a thread stopped for you">
      {stats.total === 0 ? (
        <p className="text-small text-soft">Nothing stopped for you in this range.</p>
      ) : (
        <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)] gap-6 @max-[40rem]/page:grid-cols-1 @max-[40rem]/page:gap-4">
          <div className="flex flex-col gap-3">
            <p className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <b className="text-(length:--fs-display) font-semibold tabular-nums">{stats.total.toLocaleString()}</b>
              <span className="text-small text-soft">{aDay(stats.total, days)} a day</span>
            </p>
            <ul className="flex flex-col gap-1.5 text-small">
              <li className="flex items-center gap-2 text-soft">
                <Timer size={14} className="flex-none text-dim" aria-hidden="true" />
                Median wait
                <b className="ml-auto font-medium text-ink tabular-nums">{workTime(stats.medianWaitMs, true)}</b>
              </li>
              <li className="flex items-center gap-2 text-soft">
                <Hourglass size={14} className="flex-none text-dim" aria-hidden="true" />
                Waiting on you
                <b className="ml-auto font-medium text-ink tabular-nums">{workTime(stats.totalWaitMs)}</b>
              </li>
            </ul>
            <div role="img" aria-label={`By kind: ${kinds.map((k) => `${k.count} ${KIND_LABEL[k.kind].toLowerCase()}`).join(', ')}`} className="flex h-1.5 gap-0.5">
              {kinds.map((k) => (
                <span key={k.kind} className="rounded-full" style={{ flex: k.count, background: KIND_COLOR[k.kind] }} />
              ))}
            </div>
            <ul aria-hidden="true" className="flex flex-col gap-1.5 text-small">
              {kinds.map((k) => {
                const KindIcon = KIND_ICON[k.kind]
                return (
                  <li key={k.kind} className="flex items-center gap-2">
                    <KindIcon size={14} className="flex-none" style={{ color: KIND_COLOR[k.kind] }} />
                    {KIND_LABEL[k.kind]}
                    <b className="ml-auto font-medium tabular-nums">{k.count.toLocaleString()}</b>
                  </li>
                )
              })}
            </ul>
          </div>
          <div>
            <span className="mb-1.5 block text-label font-semibold tracking-[0.08em] text-soft uppercase">Top causes</span>
            <ul aria-label="Top causes">
              {stats.byCause.slice(0, TOP_CAUSES).map((c) => (
                <StatsCause key={`${c.kind} ${c.label}`} cause={c} />
              ))}
            </ul>
          </div>
        </div>
      )}
    </StatsCard>
  )
}
