import { useLayout } from './LayoutContext'
import { useLatestOrchestrator, useOrchestrator } from './OrchestratorContext'
import type { PendingImage } from './OrchestratorContext'
import { sideChatTab } from './panelTabs'
import type { Thread } from './types'

// useSideChat starts, opens and closes side chats as tabs of their threads' right panels.
export function useSideChat() {
  const { startSideChat, deleteSideChat, report } = useOrchestrator('startSideChat', 'deleteSideChat', 'report')
  const latest = useLatestOrchestrator()
  const { showPanelView, closePanelTab } = useLayout()
  const open = (parent: string, id: string) => showPanelView(sideChatTab(id), { thread: parent })
  return {
    // start starts a side chat of the thread parent, with text and images as its first message when there are any, and
    // opens its tab.
    async start(parent: string, text = '', images: PendingImage[] = []): Promise<Thread> {
      const side = await startSideChat(parent, text, images)
      open(parent, side.id)
      return side
    },
    open,
    // close closes the side chat's tab in the open thread's panel, and deletes the side chat when nobody sent it a message.
    close(id: string) {
      closePanelTab(sideChatTab(id))
      if (latest().sideChats.some((s) => s.id === id)) report(() => deleteSideChat(id, true))
    },
  }
}
