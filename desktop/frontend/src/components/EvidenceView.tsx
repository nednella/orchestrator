import type { PanelViewProps } from '../panelViews'
import { EvidenceTab } from './EvidenceTab'
import { Notice } from './Notice'

export function EvidenceView({ session }: PanelViewProps) {
  if (!session) return <Notice title="Available in a thread" hint="Open a thread to see the media its agent showed you." centered />
  return <EvidenceTab key={session.id} session={session} />
}
