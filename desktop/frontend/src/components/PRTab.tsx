import { useAgentos } from '../AgentosContext'
import type { Session } from '../types'
import { BranchBar } from './BranchBar'
import { Chip } from './Chip'
import { Icon } from './Icon'
import { PILL, Ref } from './PRCard'
import { PRTiles } from './PRTiles'
import { isReady } from './StatusLine'
import { WorkChips } from './WorkChips'

type PRTabProps = { session: Session }

const PR_ACTION = {
  comments: { label: 'Address review comments', sentence: (n: number) => `Address the review comments on PR #${n}.` },
  checks: { label: 'Fix failing checks', sentence: (n: number) => `Fix the failing checks on PR #${n}.` },
}

export function PRTab({ session }: PRTabProps) {
  const { report, focus, typeInto, ackPR, openPR } = useAgentos('report', 'focus', 'typeInto', 'ackPR', 'openPR')
  const { pr, git } = session
  const live = pr?.state === 'draft' || pr?.state === 'open'
  const action = pr && session.prAttention ? PR_ACTION[session.prAttention] : null

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
                <Icon name={PILL[pr.state].icon} size={14} />
                {PILL[pr.state].label}
              </span>
              <span className="mono text-small text-dim">#{pr.number}</span>
            </div>
            <button
              className="text-left text-title font-semibold text-ink hover:underline"
              title={`Open #${pr.number} on GitHub`}
              onClick={() => report(() => openPR(session))}
            >
              {pr.title}
            </button>
          </>
        ) : (
          <div className="flex items-center gap-2">
            <Icon name="git-branch" size={14} />
            <span className="text-soft">No pull request yet</span>
          </div>
        )}
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-label text-dim">
          {git && (
            <span className="inline-flex min-w-0 items-center gap-1">
              <Ref>{git.base}</Ref>←<Ref>{session.branch}</Ref>
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
            <Chip tone="bad" icon="alert">
              Conflicts
            </Chip>
          )}
          {isReady(session) && (
            <Chip tone="ok" icon="check-circle-fill" title="Ready to merge. This is not an approval.">
              ready
            </Chip>
          )}
          <WorkChips session={session} alwaysWords />
        </div>
      </div>
      {pr && <PRTiles pr={pr} />}
      {pr && (
        <p className="border-t border-line px-4 py-3 text-small text-soft">
          {pr.comments === 0 ? 'No review comments.' : `${pr.comments} review ${pr.comments === 1 ? 'comment' : 'comments'}, ${pr.newComments} new.`}
        </p>
      )}
      {pr && action && (
        <div className="border-t border-line px-4 py-3">
          <button
            className="btn btn-accent"
            onClick={() => {
              report(async () => {
                await typeInto(session.id, action.sentence(pr.number))
                await ackPR(session.id)
              })
              focus('terminal')
            }}
          >
            {action.label}
          </button>
        </div>
      )}
    </div>
  )
}
