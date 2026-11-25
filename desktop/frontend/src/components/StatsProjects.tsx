import { FolderKanban } from 'lucide-react'
import { dollars } from '../spend'
import { workTime } from '../statsFigures'
import type { LedgerRow } from '../types'
import { StatsCard } from './StatsCard'

type StatsProjectsProps = { projects: LedgerRow[] }

const HEADS = ['Prompts', 'Tasks', 'Worked', 'Cost']

export function StatsProjects({ projects }: StatsProjectsProps) {
  return (
    <StatsCard icon={FolderKanban} title="Projects" meta="most prompts first">
      <div className="-mx-2">
        <table className="w-full border-separate border-spacing-0 tabular-nums">
          <thead>
            <tr>
              <th scope="col" className="h-7 px-2 text-left text-small font-medium text-soft">
                Project
              </th>
              {HEADS.map((head) => (
                <th key={head} scope="col" className="h-7 px-2 text-right text-small font-medium text-soft">
                  {head}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {projects.map((p) => (
              <tr key={p.project} className="group">
                <td className="h-9 w-full max-w-0 truncate rounded-l-lg px-2 group-hover:bg-(--bg-row-hover)">{p.project}</td>
                <td className="h-9 px-2 text-right group-hover:bg-(--bg-row-hover)">{p.prompts.toLocaleString()}</td>
                <td className="h-9 px-2 text-right group-hover:bg-(--bg-row-hover)">{p.sessions.toLocaleString()}</td>
                <td className="h-9 px-2 text-right whitespace-nowrap group-hover:bg-(--bg-row-hover)">{workTime(p.workMs)}</td>
                <td className="h-9 rounded-r-lg px-2 text-right whitespace-nowrap group-hover:bg-(--bg-row-hover)">~{dollars(p.cost)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </StatsCard>
  )
}
