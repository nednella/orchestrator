import type { PanelViewProps } from '../panelViews'
import { useThreadServers } from '../useThreadServers'
import { Notice } from './Notice'
import { ServersTab } from './ServersTab'

export function ServersView({ session }: PanelViewProps) {
  const { servers, refresh } = useThreadServers(session?.id ?? '', session?.state, true)
  if (!session) return <Notice title="Available in a thread" hint="Open a thread to see the servers it runs." centered />
  return <ServersTab session={session} servers={servers} refresh={refresh} />
}
