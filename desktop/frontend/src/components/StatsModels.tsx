import { Cpu } from 'lucide-react'
import { shortModel } from '../model'
import { dollars, tokenCount } from '../spend'
import { shortDay } from '../statsChart'
import type { Ledger } from '../types'
import { StatsCard } from './StatsCard'

type StatsModelsProps = { usage: Ledger['usage'] }

const HEADS = ['Tasks', 'Tokens', 'Cost']

export function StatsModels({ usage }: StatsModelsProps) {
  const { totals, byModel, pricesAsOf } = usage
  const cached = totals.tokens ? Math.round((totals.cacheRead / totals.tokens) * 100) : 0
  return (
    <StatsCard icon={Cpu} title="By model" meta="tasks and chats">
      {byModel.length === 0 ? (
        <p className="text-small text-soft">No usage in this range.</p>
      ) : (
        <div className="-mx-2">
          <table className="w-full border-separate border-spacing-0 tabular-nums">
            <thead>
              <tr>
                <th scope="col" className="h-7 px-2 text-left text-small font-medium text-soft">
                  Model
                </th>
                {HEADS.map((head) => (
                  <th key={head} scope="col" className="h-7 px-2 text-right text-small font-medium text-soft">
                    {head}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {byModel.map((r) => (
                <tr key={`${r.model} ${r.effort}`} className="group">
                  <td className="h-9 rounded-l-lg px-2 group-hover:bg-(--bg-row-hover)" title={r.model}>
                    <span className="flex items-baseline gap-1.5">
                      <span>{shortModel(r.model) || 'unknown'}</span>
                      <span className="text-small text-soft">{r.effort || 'default'}</span>
                    </span>
                  </td>
                  <td className="h-9 px-2 text-right group-hover:bg-(--bg-row-hover)">{r.sessions.toLocaleString()}</td>
                  <td className="h-9 px-2 text-right group-hover:bg-(--bg-row-hover)">{tokenCount(r.tokens, true)}</td>
                  <td className="h-9 rounded-r-lg px-2 text-right group-hover:bg-(--bg-row-hover)">~{dollars(r.cost)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <p className="mt-3 text-small leading-normal text-soft">
        {cached > 0 && `${cached}% of tokens are cache reads. `}Estimated at list prices of {shortDay(pricesAsOf)}; subagents included.
      </p>
    </StatsCard>
  )
}
