import type { Issue } from '../types'

type IssueChipsProps = { issues: Issue[]; picked: number | null; onPick(number: number): void }

export function IssueChips({ issues, picked, onPick }: IssueChipsProps) {
  if (issues.length === 0) return null
  return (
    <div className="mt-3.5 flex flex-wrap justify-center gap-1.5" aria-label="Top issues">
      {issues.map((issue) => (
        <button
          key={issue.number}
          className="inline-flex h-7 max-w-[14rem] min-w-0 items-center gap-1.5 rounded-full border border-line px-2.5 text-small text-soft hover:bg-hover hover:text-ink aria-pressed:border-line-strong aria-pressed:text-ink"
          title={`Pick #${issue.number} ${issue.title}`}
          aria-pressed={picked === issue.number}
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => onPick(issue.number)}
        >
          <span className="flex-none text-dim tabular-nums">#{issue.number}</span>
          <span className="truncate">{issue.title}</span>
        </button>
      ))}
    </div>
  )
}
