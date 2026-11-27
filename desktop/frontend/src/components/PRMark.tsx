import { PR_ICON } from '../prState'
import type { PR } from '../types'
import { ChecksMark } from './ChecksMark'

type PRMarkProps = { pr: PR }

export function PRMark({ pr }: PRMarkProps) {
  const Shape = PR_ICON[pr.state]
  return (
    <span className="flex flex-none items-center gap-1" title={`PR #${pr.number}, ${pr.state}, checks ${pr.checks}`}>
      <Shape size={14} color={`var(--pr-${pr.state})`} aria-label={`Pull request ${pr.state}`} role="img" />
      <ChecksMark checks={pr.checks} />
    </span>
  )
}
