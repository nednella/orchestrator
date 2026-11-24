import type { Issue } from '../types'
import { TypeMark } from './TypeMark'

type IssuePickListProps = { issues: Issue[]; cursor: number; onPick(number: number): void }

export function IssuePickList({ issues, cursor, onPick }: IssuePickListProps) {
  if (issues.length === 0) return <p className="px-2 py-1.5 text-label text-dim">No open issue matches.</p>
  return (
    <div role="listbox" aria-label="Issues" className="flex max-h-72 flex-col gap-0.5 overflow-y-auto">
      {issues.map((issue, i) => (
        <button
          key={issue.number}
          role="option"
          aria-selected={i === cursor}
          data-selected={i === cursor}
          className="composer-option"
          onMouseDown={(e) => {
            e.preventDefault()
            onPick(issue.number)
          }}
        >
          <TypeMark type={issue.type} />
          <span className="mono flex-none text-label text-dim">{issue.number}</span>
          <span className="min-w-0 flex-1 truncate text-label font-medium">{issue.title}</span>
          {issue.sessionId && <span className="flex-none text-label text-dim">has a thread</span>}
        </button>
      ))}
    </div>
  )
}
