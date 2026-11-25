import { useState } from 'react'
import { ArrowDown, ChevronRight, CircleDot } from 'lucide-react'
import { dollars, tokenCount } from '../spend'
import { counted, workTitle } from '../statsFigures'
import { shortAgo } from '../threads'
import { useClock } from '../time'
import type { WorkRow } from '../types'
import { StatsCard } from './StatsCard'

type StatsWorkProps = { work: WorkRow[]; showProject: boolean }

type SortKey = 'sessions' | 'tokens' | 'cost' | 'lastAt'

const ROWS = 8

const COLUMNS: { key: SortKey; label: string; wide?: boolean }[] = [
  { key: 'sessions', label: 'Tasks' },
  { key: 'tokens', label: 'Tokens', wide: true },
  { key: 'cost', label: 'Cost' },
  { key: 'lastAt', label: 'Last', wide: true },
]

const NARROW_HIDDEN = '@max-[40rem]/page:hidden'

export function StatsWork({ work, showProject }: StatsWorkProps) {
  const [sortKey, setSortKey] = useState<SortKey>('cost')
  const [all, setAll] = useState(false)
  const now = useClock((t) => Math.floor(t / 60_000) * 60_000)
  const topCost = Math.max(0, ...work.map((w) => w.cost))
  const rows = [...work].sort((a, b) => b[sortKey] - a[sortKey]).slice(0, all ? undefined : ROWS)
  const more =
    work.length > ROWS ? (
      <button className="inline-flex h-6 items-center gap-1.5 rounded-lg px-1.5 text-small font-medium whitespace-nowrap text-soft hover:bg-(--bg-ghost-hover) hover:text-ink" aria-expanded={all} onClick={() => setAll(!all)}>
        {all ? 'Show fewer' : 'Show all'}
        <ChevronRight size={12} aria-hidden="true" className={all ? '-rotate-90' : ''} />
      </button>
    ) : undefined
  return (
    <StatsCard icon={CircleDot} title="Work" meta={counted(work.length, 'issue or thread', 'issues and threads')} end={more}>
      {work.length === 0 ? (
        <p className="text-small text-soft">No usage in this range.</p>
      ) : (
        <div className="-mx-2">
          <table className="w-full border-separate border-spacing-0 tabular-nums">
            <thead>
              <tr>
                <th scope="col" className="h-7 px-2 text-left text-small font-medium whitespace-nowrap text-soft">
                  Issue or thread
                </th>
                {COLUMNS.map((c) => (
                  <th key={c.key} scope="col" aria-sort={sortKey === c.key ? 'descending' : 'none'} className={`h-7 px-2 text-right text-small font-medium whitespace-nowrap text-soft ${c.wide ? NARROW_HIDDEN : ''}`}>
                    <button className={`inline-flex items-center gap-1 font-medium hover:text-ink ${sortKey === c.key ? 'text-ink' : ''}`} title={c.key === 'lastAt' ? 'Sort by latest first' : `Sort by ${c.label.toLowerCase()}, highest first`} onClick={() => setSortKey(c.key)}>
                      {c.label}
                      {sortKey === c.key && <ArrowDown size={12} aria-hidden="true" />}
                    </button>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((w) => {
                const { ref, title } = workTitle(w)
                return (
                  <tr key={`${w.project} ${w.issue} ${w.session}`} className="group">
                    <td className="h-9 w-full max-w-0 truncate rounded-l-lg px-2 group-hover:bg-(--bg-row-hover)" title={w.title}>
                      {ref && <span className="text-soft">{ref} </span>}
                      {title}
                      {showProject && <span className="text-soft"> · {w.project}</span>}
                    </td>
                    <td className="h-9 px-2 text-right whitespace-nowrap group-hover:bg-(--bg-row-hover)">{w.sessions.toLocaleString()}</td>
                    <td className={`h-9 px-2 text-right whitespace-nowrap group-hover:bg-(--bg-row-hover) ${NARROW_HIDDEN}`}>{tokenCount(w.tokens, true)}</td>
                    <td className="h-9 px-2 text-right whitespace-nowrap group-hover:bg-(--bg-row-hover)">
                      <span className="inline-grid grid-cols-[3rem_4.5rem] items-center gap-2">
                        <span aria-hidden="true" className="h-1 overflow-hidden rounded-full bg-(--bg-segment)">
                          <span className="block h-full rounded-full bg-(--chart-bar-on)" style={{ width: `${topCost > 0 ? (w.cost / topCost) * 100 : 0}%` }} />
                        </span>
                        ~{dollars(w.cost)}
                      </span>
                    </td>
                    <td className={`h-9 rounded-r-lg px-2 text-right whitespace-nowrap text-soft group-hover:bg-(--bg-row-hover) ${NARROW_HIDDEN}`}>{shortAgo(w.lastAt, now)}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </StatsCard>
  )
}
