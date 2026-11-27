import type { PanelViewProps } from '../panelViews'
import { BrowserTab } from './BrowserTab'
import { Notice } from './Notice'

export function BrowserView({ thread }: PanelViewProps) {
  if (!thread) return <Notice title="Available in a thread" hint="Open a thread to see its browser page." centered />
  return <BrowserTab key={thread.id} thread={thread} />
}
