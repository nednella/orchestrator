import { useEffect, useRef, useState } from 'react'
import { Calendar, ChartColumn, Eraser } from 'lucide-react'
import { api } from '../api'
import { useAgentos } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { rangeLabel } from '../statsChart'
import { periodBefore } from '../statsFigures'
import { readStored, writeStored } from '../storage'
import { useMinimumLoading } from '../useMinimumLoading'
import type { Ledger } from '../types'
import { CleanupsList } from './CleanupsList'
import { Loading } from './Loading'
import { MemoryTotal } from './MemoryTotal'
import { SegmentedControl } from './SegmentedControl'
import { StatsCard } from './StatsCard'
import { StatsFigures } from './StatsFigures'
import { StatsInterruptions } from './StatsInterruptions'
import { StatsModels } from './StatsModels'
import { StatsPerDay } from './StatsPerDay'
import { StatsProjectPicker } from './StatsProjectPicker'
import { StatsProjects } from './StatsProjects'
import { StatsWork } from './StatsWork'

const RANGES = [
  { value: '1', label: 'Today' },
  { value: '7', label: '7d' },
  { value: '30', label: '30d' },
  { value: '90', label: '90d' },
  { value: '365', label: '1y' },
  { value: '0', label: 'All' },
]
const RANGE_KEY = 'agentos.statsWorkDays'
const PROJECT_KEY = 'agentos.statsProject'

export function Stats() {
  const { focusRequest, projects, project: currentProject, report } = useAgentos('focusRequest', 'projects', 'project', 'report')
  const { closeCentre } = useLayout()
  const [days, setDays] = useState(() => {
    const stored = readStored(RANGE_KEY, 30)
    return RANGES.some((r) => r.value === String(stored)) ? stored : 30
  })
  const [picked, setPicked] = useState(() => readStored(PROJECT_KEY, ''))
  const [ledger, setLedger] = useState<Ledger | null>(null)
  const panel = useRef<HTMLElement>(null)
  const project = projects.some((p) => p.key === picked) ? picked : ''
  const loading = useMinimumLoading(ledger === null)

  useEffect(() => {
    if (focusRequest.target === 'thread') panel.current?.focus()
  }, [focusRequest])

  useEffect(() => panel.current?.focus(), [])

  useEffect(() => {
    let current = true
    report(async () => {
      const next = await api.ledger(days, project)
      if (current) setLedger(next)
    })
    return () => {
      current = false
    }
  }, [days, project, report])

  const pickRange = (next: string) => {
    setDays(Number(next))
    writeStored(RANGE_KEY, Number(next))
  }
  const pickProject = (next: string) => {
    setPicked(next)
    writeStored(PROJECT_KEY, next)
  }

  const first = ledger?.perDay[0]?.day
  const last = ledger?.perDay.at(-1)?.day
  const period = periodBefore(ledger?.days ?? days)

  return (
    <section
      ref={panel}
      data-panel="thread"
      tabIndex={-1}
      aria-labelledby="stats-title"
      className="panel @container/page h-full min-h-0 min-w-0 overflow-y-auto [scrollbar-gutter:stable]"
      onKeyDown={(e) => e.key === 'Escape' && closeCentre()}
    >
      <div className="mx-auto w-full max-w-[46rem] px-4 pt-6 pb-16 @max-[40rem]/page:pt-4">
        <header className="flex flex-wrap items-center gap-3">
          <span className="grid size-8 flex-none place-items-center rounded-lg bg-(--bg-tile) text-soft shadow-[inset_0_0_0_1px_var(--border)]">
            <ChartColumn size={16} aria-hidden="true" />
          </span>
          <div className="flex min-w-48 flex-1 flex-col">
            <h1 id="stats-title" className="text-title leading-[1.375] font-semibold">
              Stats
            </h1>
            <p className="text-small text-soft">{project ? 'What the agents did in this project' : 'What the agents did in every project'}</p>
          </div>
          <SegmentedControl variant="track" label="Range" options={RANGES} value={String(days)} onChange={pickRange} />
        </header>
        <div className="mt-4 flex min-h-6 flex-wrap items-center gap-2 border-b border-(--border-rule) px-1 pt-1 pb-2 text-body text-soft tabular-nums">
          <Calendar size={16} aria-hidden="true" className="flex-none text-dim" />
          {first && last && <span>{rangeLabel(first, last)}</span>}
          {period && <span className="text-small whitespace-nowrap">compared with {period}</span>}
          <span className="ml-auto flex items-center gap-2">
            <MemoryTotal />
            <StatsProjectPicker projects={projects} value={project} onChange={pickProject} />
          </span>
        </div>
        {loading || ledger === null ? (
          <Loading />
        ) : ledger.since === '' ? (
          <p className="mt-5 text-center text-small text-soft">Nothing recorded yet. Chats, tasks, prompts, worked time and tokens are counted from now on.</p>
        ) : (
          <div className="mt-4 flex flex-col gap-5">
            <StatsFigures ledger={ledger} />
            <StatsPerDay days={ledger.perDay} />
            <StatsInterruptions days={days || ledger.perDay.length} project={project} />
            <StatsWork work={ledger.work} showProject={!project} />
            <StatsModels usage={ledger.usage} />
            {!project && ledger.projects.length > 1 && <StatsProjects projects={ledger.projects} />}
            <StatsCard icon={Eraser} title="Cleaned up" meta={currentProject ? `tasks cleaned up in ${currentProject.name}` : undefined}>
              <CleanupsList />
            </StatsCard>
          </div>
        )}
      </div>
    </section>
  )
}
