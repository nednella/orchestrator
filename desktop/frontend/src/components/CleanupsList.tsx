import { useOrchestrator } from '../OrchestratorContext'
import { cleanupLine } from '../cleanupText'
import { Ago } from './Ago'

export function CleanupsList() {
  const { cleanups } = useOrchestrator('cleanups')
  if (cleanups.length === 0) return <p className="text-small text-dim">Nothing cleaned up yet.</p>
  return (
    <div className="flex max-w-3xl flex-col">
      {cleanups.map((c) => (
        <div key={`${c.at}-${c.sessionTitle}`} className="border-b border-line py-2.5 last:border-b-0">
          <div className="flex items-baseline gap-2">
            <span className="min-w-0 flex-1 truncate text-body font-medium">{c.sessionTitle}</span>
            {c.pr > 0 && <span className="mono text-small text-dim">PR #{c.pr}</span>}
            <span className="mono text-label text-dim">
              <Ago at={c.at} />
            </span>
          </div>
          <p className={`text-small ${c.status === 'blocked' ? 'text-danger' : 'text-dim'}`}>{cleanupLine(c)}</p>
        </div>
      ))}
    </div>
  )
}
