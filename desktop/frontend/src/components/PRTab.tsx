import { ArrowLeft, CircleCheck, GitBranch, GitMerge, GitPullRequestArrow, GitPullRequestClosed, GitPullRequestDraft, TriangleAlert } from 'lucide-react'
import { useOrchestrator } from '../OrchestratorContext'
import { useLayout } from '../LayoutContext'
import type { Thread } from '../types'
import { BranchBar } from './BranchBar'
import { Chip } from './Chip'
import { PRTiles } from './PRTiles'
import { Ref } from './Ref'
import { WorkChips } from './WorkChips'

type PRTabProps = { thread: Thread }

const PILL = {
  open: { icon: GitPullRequestArrow, label: 'Open' },
  draft: { icon: GitPullRequestDraft, label: 'Draft' },
  merged: { icon: GitMerge, label: 'Merged' },
  closed: { icon: GitPullRequestClosed, label: 'Closed' },
}

function isReady({ pr, git }: Thread): boolean {
  const live = pr?.state === 'draft' || pr?.state === 'open'
  const passing = pr?.checks === 'passing' || pr?.checks === 'none'
  return live && passing && !pr.conflicting && pr.newComments === 0 && git?.unpushed === 0 && git.uncommitted === 0
}

const PR_ACTION = {
  comments: { label: 'Address review comments', sentence: (n: number) => `Address the review comments on PR #${n}.` },
  checks: { label: 'Fix failing checks', sentence: (n: number) => `Fix the failing checks on PR #${n}.` },
}

export function PRTab({ thread }: PRTabProps) {
  const { report, focus, select, typeInto, ackPR, openPR } = useOrchestrator('report', 'focus', 'select', 'typeInto', 'ackPR', 'openPR')
  const { closeOverlay } = useLayout()
  const { pr, git } = thread
  const live = pr?.state === 'draft' || pr?.state === 'open'
  const pill = PILL[pr?.state ?? 'open']
  const action = pr && thread.prAttention ? PR_ACTION[thread.prAttention] : null

  if (!pr && !git) {
    return (
      <div className="grid h-full place-items-center p-6 text-center">
        <p className="max-w-xs text-soft">This thread has no branch yet. Its pull request appears here once the agent opens one.</p>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col overflow-y-auto">
      <div className="flex min-w-0 flex-col gap-2 px-4 pt-4 pb-3">
        {pr ? (
          <>
            <div className="flex items-center gap-2">
              <span
                className="inline-flex h-[22px] flex-none items-center gap-1 rounded-full pr-2 pl-1.5 text-label font-medium text-white"
                style={{ background: `var(--pr-${pr.state})` }}
              >
                <pill.icon size={14} aria-hidden="true" />
                {pill.label}
              </span>
              <span className="text-small text-dim tabular-nums">#{pr.number}</span>
            </div>
            <button
              className="text-left text-title font-semibold text-ink hover:underline"
              title={`Open #${pr.number} on GitHub`}
              onClick={() => report(() => openPR(thread))}
            >
              {pr.title}
            </button>
          </>
        ) : (
          <div className="flex items-center gap-2">
            <GitBranch size={14} aria-hidden="true" />
            <span className="text-soft">No pull request yet</span>
          </div>
        )}
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-label text-dim">
          {git && (
            <span className="inline-flex min-w-0 items-center gap-1">
              <Ref>{git.base}</Ref>
              <ArrowLeft size={12} aria-hidden="true" />
              <Ref>{thread.branch}</Ref>
            </span>
          )}
          {git && (
            <span className="ml-auto">
              <BranchBar ahead={git.ahead} behind={git.behind} />
            </span>
          )}
        </div>
        <div className="flex flex-wrap gap-1.5 empty:hidden">
          {live && pr.conflicting && (
            <Chip tone="bad" icon={TriangleAlert}>
              Conflicts
            </Chip>
          )}
          {isReady(thread) && (
            <Chip tone="ok" icon={CircleCheck} title="Ready to merge. This is not an approval.">
              Ready
            </Chip>
          )}
          <WorkChips thread={thread} />
        </div>
      </div>
      {pr && <PRTiles pr={pr} />}
      {pr && (
        <p className="border-t border-(--border-rule) px-4 py-3 text-small text-soft">
          {pr.comments === 0 ? 'No review comments.' : `${pr.comments} review ${pr.comments === 1 ? 'comment' : 'comments'}, ${pr.newComments} new.`}
        </p>
      )}
      {pr && action && (
        <div className="border-t border-(--border-rule) px-4 py-3">
          <button
            className="btn btn-accent"
            onClick={() => {
              report(async () => {
                await typeInto(thread.id, action.sentence(pr.number))
                await ackPR(thread.id)
              })
              select(thread.id)
              closeOverlay('thread-focused')
              focus('composer')
            }}
          >
            {action.label}
          </button>
        </div>
      )}
    </div>
  )
}
