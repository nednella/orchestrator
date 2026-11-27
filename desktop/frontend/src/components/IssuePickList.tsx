import type { Issue } from '../types'
import { ComposerPickList } from './ComposerPickList'
import { TypeMark } from './TypeMark'

type IssuePickListProps = { issues: Issue[]; cursor: number; onPick(number: number): void }

export function IssuePickList({ issues, cursor, onPick }: IssuePickListProps) {
  const rows = issues.map((issue) => ({
    key: String(issue.number),
    icon: (
      <>
        <TypeMark type={issue.type} />
        <span className="mono flex-none text-label text-dim">{issue.number}</span>
      </>
    ),
    name: issue.title,
    badge: issue.threadId ? 'has a thread' : undefined,
  }))
  return <ComposerPickList label="Issues" rows={rows} cursor={cursor} empty="No open issue matches." onPick={(i) => onPick(issues[i].number)} />
}
