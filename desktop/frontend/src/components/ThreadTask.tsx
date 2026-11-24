import { currentTask } from '../threads'
import type { TaskTone } from '../threads'
import type { Session } from '../types'
import { Icon } from './Icon'

type ThreadTaskProps = { session: Session }

const TONE: Record<TaskTone, string> = {
  pending: 'var(--text-soft)',
  passing: 'var(--finished)',
  failing: 'var(--danger)',
  merged: 'var(--pr-merged)',
  closed: 'var(--text-dim)',
}

export function ThreadTask({ session }: ThreadTaskProps) {
  const task = currentTask(session)
  if (!task) return null
  if (task.kind === 'issue') return <span className="flex-none text-soft">#{task.number}</span>
  const { pr } = session
  return (
    <span className="flex flex-none items-center gap-1 tabular-nums" style={{ color: TONE[task.tone] }} title={pr ? `PR #${pr.number}, ${pr.state}, checks ${pr.checks}` : undefined}>
      <Icon name="pull-request" size={12} />
      {task.number}
    </span>
  )
}
