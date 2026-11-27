import { Asterisk, CircleDot, GitBranch, GitMerge, GitPullRequestArrow, GitPullRequestClosed, GitPullRequestDraft, Laptop } from 'lucide-react'
import type { ReactNode } from 'react'
import type { CardIcon } from '../threadCard'

const ICONS = {
  machine: Laptop,
  'git-branch': GitBranch,
  'git-pull-request': GitPullRequestArrow,
  'git-pull-request-draft': GitPullRequestDraft,
  'git-merge': GitMerge,
  'git-pull-request-closed': GitPullRequestClosed,
  'circle-dot': CircleDot,
  claude: Asterisk,
}

type ThreadCardLineProps = { icon: CardIcon; iconColor?: string; children: ReactNode }

export function ThreadCardLine({ icon, iconColor, children }: ThreadCardLineProps) {
  const Icon = ICONS[icon]
  return (
    <div className="flex min-w-0 items-start gap-2">
      <span className="flex h-5 flex-none items-center" style={{ color: iconColor ?? 'var(--text-dim)' }}>
        <Icon size={14} aria-hidden="true" />
      </span>
      <span className="min-w-0 break-words">{children}</span>
    </div>
  )
}
