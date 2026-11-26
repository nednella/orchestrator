import { useEffect, useMemo, useRef, useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { filterCounts, groupByTool } from '../digestFeed'
import type { DigestFilter } from '../digestFeed'
import { runState } from '../digestStatus'
import { DigestEmpty } from './DigestEmpty'
import { DigestFailure } from './DigestFailure'
import { DigestFilters } from './DigestFilters'
import { DigestHeader } from './DigestHeader'
import { DigestRunLines } from './DigestRunLines'
import { DigestStatusRow } from './DigestStatusRow'
import { DigestToolCard } from './DigestToolCard'
import { DigestToolsMenu } from './DigestToolsMenu'
import { CenteredSpinner } from './CenteredSpinner'
import { Notice } from './Notice'

const NONE_LEFT: Record<DigestFilter, string> = { all: 'No items', new: 'No new items', saved: 'No saved items' }

export function DigestView() {
  const { digest, digestError, reloadDigest, digestSeen, runDigest, markDigestSeen, report, project } = useOrchestrator('digest', 'digestError', 'reloadDigest', 'digestSeen', 'runDigest', 'markDigestSeen', 'report', 'project')
  const loading = !digest
  const panel = useRef<HTMLElement>(null)
  const [filter, setFilter] = useState<DigestFilter>('all')
  // What counts as new is fixed when the page opens on a project, before opening marks everything seen.
  const [seen, setSeen] = useState<{ project: string; at: number } | null>(null)
  if (digest && seen?.project !== digest.project) setSeen({ project: digest.project, at: digestSeen })
  const seenAt = seen?.at ?? digestSeen
  const items = digest?.items
  const counts = useMemo(() => filterCounts(items ?? [], seenAt), [items, seenAt])
  const tools = useMemo(() => groupByTool(items ?? [], filter, seenAt), [items, filter, seenAt])
  const loaded = digest !== null

  useEffect(() => {
    if (loaded) markDigestSeen()
  }, [loaded, digest?.project, digest?.lastRunAt, markDigestSeen])

  const run = () => report(runDigest)
  const showTool = (key: string) => panel.current?.querySelector(`[data-tool="${CSS.escape(key)}"]`)?.scrollIntoView({ block: 'start', behavior: 'smooth' })
  const state = digest ? runState(digest) : null
  const empty = (digest?.items.length ?? 0) === 0
  const button = loading ? null : state === 'running' ? 'checking' : state === 'checked' && !empty ? 'run' : null

  return (
    <section ref={panel} aria-label="News" className="@container absolute inset-0 overflow-y-auto [scrollbar-gutter:stable]">
      <div className="mx-auto w-full max-w-(--chat-column) px-4 pt-6 pb-16 @max-[40rem]:px-3 @max-[40rem]:pt-3 @max-[40rem]:pb-10">
        <DigestHeader project={project?.name ?? 'this project'} tools={digest?.tools ?? 0} automatic={(digest?.nextRunAt ?? 0) > 0} button={button} onRun={run} />
        {loading && digestError && (
          <Notice
            title="The news did not load"
            hint={
              <>
                {digestError}{' '}
                <button className="btn btn-ghost" onClick={reloadDigest}>
                  Retry
                </button>
              </>
            }
          />
        )}
        {loading && !digestError && <CenteredSpinner />}
        {!loading && digest && (
          <>
            <DigestStatusRow digest={digest} />
            {state === 'running' && <DigestRunLines found={digest.items.filter((i) => i.at >= digest.startedAt)} />}
            {state === 'failed' && <DigestFailure error={digest.error} onRun={run} />}
            {(state === 'never' || state === 'checked') && empty && <DigestEmpty onRun={run} />}
            {!empty && (
              <>
                <div className="mt-4 mb-3 flex items-center justify-between gap-2 @max-[40rem]:mt-3 @max-[40rem]:mb-2">
                  <DigestFilters value={filter} counts={counts} onChange={setFilter} />
                  <DigestToolsMenu tools={tools} onPick={showTool} />
                </div>
                <div className="flex flex-col gap-5 @max-[40rem]:gap-3">
                  {tools.map((tool) => (
                    <DigestToolCard key={tool.key} tool={tool} seen={seenAt} />
                  ))}
                </div>
                <p className="mt-5 text-center text-small text-soft @max-[40rem]:mt-4">
                  {tools.length === 0 ? NONE_LEFT[filter] : `${counts.all} ${counts.all === 1 ? 'item' : 'items'}. Older ones drop off as new ones come; save one as a note to keep it.`}
                </p>
              </>
            )}
          </>
        )}
      </div>
    </section>
  )
}
