import type { PanelViewProps } from '../panelViews'
import { Notice } from './Notice'
import { PRTab } from './PRTab'

export function PullRequestsView({ session }: PanelViewProps) {
  if (!session) return <Notice title="Available in a thread" hint="Open a thread to see its pull request." centered />
  return <PRTab key={session.id} session={session} />
}
