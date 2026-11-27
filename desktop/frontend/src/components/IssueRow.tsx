import { useState } from 'react'
import { useOrchestratorSelector } from '../OrchestratorContext'
import type { Issue } from '../types'
import type { IssueDrag } from '../useIssueDrag'
import { IssueThread } from './IssueThread'
import { IssueWorkflow } from './IssueWorkflow'
import { ModelTag } from './ModelTag'
import { PRMark } from './PRMark'
import { TypeMark } from './TypeMark'

type IssueRowProps = { issue: Issue; drag: IssueDrag; onRead(): void }

export function IssueRow({ issue, drag, onRead }: IssueRowProps) {
  const thread = useOrchestratorSelector((a) => a.projectThreads.find((s) => s.id === issue.threadId))
  const [menuOpen, setMenuOpen] = useState(false)
  const labelled = (key: string) => issue.labels.find((l) => l.startsWith(`${key}:`))?.slice(key.length + 1) ?? ''

  return (
    <div
      className="row row-inset issue-row group flex-col gap-1 px-3 py-2"
      data-issue={issue.number}
      data-menu-open={menuOpen}
      draggable={issue.moves.length > 0}
      onDragStart={(e) => {
        e.dataTransfer.effectAllowed = 'move'
        e.dataTransfer.setData('text/plain', String(issue.number))
        drag.start(issue)
      }}
      onDragEnd={drag.end}
      onClick={(e) => {
        // A click in the ⋯ menu reaches the row through React although the menu sits outside it in the page.
        const target = e.target as Element
        if (e.currentTarget.contains(target) && !target.closest('button')) onRead()
      }}
    >
      <button data-nav className="flex min-w-0 items-center gap-2 text-left" title="Read the issue" onClick={onRead}>
        <span className="min-w-0 flex-1 truncate text-body">
          <span className="text-soft">#{issue.number}</span> {issue.title}
        </span>
        {thread?.pr && <PRMark pr={thread.pr} />}
      </button>
      <div className="flex min-w-0 items-center gap-3 text-small text-dim empty:hidden">
        {issue.type && <TypeMark type={issue.type} named />}
        <ModelTag model={labelled('model')} effort={labelled('effort')} problem={issue.ignored.join('\n')} className="min-w-0 truncate" />
        {thread && <IssueThread thread={thread} />}
      </div>
      {!thread && (
        <div className="issue-actions pt-1">
          <IssueWorkflow issue={issue} onMenu={setMenuOpen} />
        </div>
      )}
    </div>
  )
}
