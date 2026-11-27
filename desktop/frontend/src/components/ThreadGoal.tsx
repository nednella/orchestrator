import { GitPullRequest } from 'lucide-react'
import { threadGoal } from '../threads'
import type { GoalTone } from '../threads'
import type { Thread } from '../types'

type ThreadGoalProps = { thread: Thread }

const TONE: Record<GoalTone, string> = {
  pending: 'var(--text-soft)',
  passing: 'var(--finished)',
  failing: 'var(--danger)',
  merged: 'var(--pr-merged)',
  closed: 'var(--text-dim)',
}

export function ThreadGoal({ thread }: ThreadGoalProps) {
  const goal = threadGoal(thread)
  if (!goal) return null
  if (goal.kind === 'issue') return <span className="flex-none text-soft">#{goal.number}</span>
  const { pr } = thread
  return (
    <span className="flex flex-none items-center gap-1 tabular-nums" style={{ color: TONE[goal.tone] }} title={pr ? `PR #${pr.number}, ${pr.state}, checks ${pr.checks}` : undefined}>
      <GitPullRequest size={12} aria-hidden="true" />
      {goal.number}
    </span>
  )
}
