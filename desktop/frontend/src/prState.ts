import { GitMerge, GitPullRequestArrow, GitPullRequestClosed, GitPullRequestDraft } from 'lucide-react'
import type { PR } from './types'

export const PR_ICON = { open: GitPullRequestArrow, draft: GitPullRequestDraft, closed: GitPullRequestClosed, merged: GitMerge }

export const PR_STATE: Record<PR['state'], string> = { open: 'Open', draft: 'Draft', closed: 'Closed', merged: 'Merged' }
