import { gigabytes } from '../memory'
import { dollars, tokenCount } from '../spend'
import type { Session } from '../types'
import { ActionTag } from './ActionTag'
import { ChatTag } from './ChatTag'
import { ModelTag } from './ModelTag'
import { StateBadge } from './StateBadge'

type ThreadFactsProps = { session: Session }

export function ThreadFacts({ session }: ThreadFactsProps) {
  return (
    <div className="flex flex-none flex-col gap-2 border-b border-line px-4 py-3">
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <span className="mono text-small font-semibold text-soft" title="Thread number">
          {session.n}
        </span>
        <StateBadge state={session.state} chat={session.chat} />
        <ActionTag action={session.action} />
        {session.chat && <ChatTag />}
      </div>
      <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
        <ModelTag
          className="min-w-0 truncate"
          agent={session.agent}
          model={session.model}
          effort={session.effort}
          spend={session.tokens > 0 ? `${tokenCount(session.tokens)} tokens · ~${dollars(session.cost)}` : ''}
        />
        {session.memory > 0 && (
          <span className="mono text-small whitespace-nowrap text-dim" title="Memory this thread's processes use">
            {gigabytes(session.memory)}
          </span>
        )}
      </div>
    </div>
  )
}
