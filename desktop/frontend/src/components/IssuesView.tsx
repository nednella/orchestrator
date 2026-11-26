import { useCallback, useEffect, useRef, useState } from 'react'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { api } from '../api'
import { filterIssues, isFiltering } from '../issueFilter'
import { useFocusRequest } from '../useFocusRequest'
import { useRefreshQueueOnShow } from '../useRefreshQueueOnShow'
import { IssueList } from './IssueList'
import { IssueReader } from './IssueReader'
import { Notice } from './Notice'
import { QueueSearch } from './QueueSearch'

// IssuesView is the queue's list, and an issue read in its place, or beside it where the panel is 720 px wide or more.
export function IssuesView() {
  const { issues, issuesDisabled, issueFilter, report } = useAgentos('issues', 'issuesDisabled', 'issueFilter', 'report')
  const projectName = useAgentosSelector((a) => a.project?.name)
  const repo = useAgentosSelector((a) => a.project?.repo ?? '')
  const root = useRef<HTMLDivElement>(null)
  const [reading, setReading] = useState<number | null>(null)

  useRefreshQueueOnShow()

  // Searching or filtering the queue from a shortcut or a command shows the list again.
  const closeReader = useCallback(() => setReading(null), [])
  useFocusRequest('queue-filter', closeReader)
  useEffect(closeReader, [issueFilter, closeReader])

  if (projectName !== undefined && !repo) {
    return <Notice title="No repo linked" hint={`${projectName} has no GitHub repo, so it has no issues.`} centered />
  }
  if (projectName !== undefined && issuesDisabled) {
    const link = (
      <button className="link" onClick={() => report(() => api.openURL(`https://github.com/${repo}`))}>
        {repo}
      </button>
    )
    return <Notice title="Issues are disabled" hint={<>{link} has issues disabled, enable them to use the queue feature</>} centered />
  }
  const filtering = isFiltering(issueFilter)
  const shown = filtering ? filterIssues(issues, issueFilter) : issues

  const back = () => {
    const number = reading
    setReading(null)
    requestAnimationFrame(() => root.current?.querySelector<HTMLElement>(`[data-issue="${number}"] [data-nav]`)?.focus())
  }

  return (
    <div ref={root} className="flex h-full min-h-0">
      <div className={`min-h-0 min-w-0 flex-1 flex-col ${reading === null ? 'flex' : 'hidden @min-[720px]:flex'}`}>
        <QueueSearch shown={filtering ? shown.length : null} />
        <IssueList shown={shown} filtering={filtering} onRead={setReading} />
      </div>
      {reading !== null && (
        <div className="flex min-h-0 min-w-0 flex-1 flex-col border-line @min-[720px]:border-l">
          <IssueReader key={reading} number={reading} onBack={back} />
        </div>
      )}
    </div>
  )
}
