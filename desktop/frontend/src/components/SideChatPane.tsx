import { Trash2, X } from 'lucide-react'
import { useState } from 'react'
import { useLayout } from '../LayoutContext'
import { useOrchestrator, useOrchestratorSelector } from '../OrchestratorContext'
import { errorMessage } from '../api'
import { askConfirm } from '../confirm'
import { sideChatTab } from '../panelTabs'
import { SIDE_CHAT_TITLE } from '../sideChat'
import { attach, detach } from '../sideChatStore'
import { useDelayedTip } from '../useDelayedTip'
import { useSideChat } from '../useSideChat'
import { ChatView } from './ChatView'
import { FloatingTip } from './FloatingTip'

type SideChatPaneProps = { id: string }

const iconButton = 'btn btn-ghost h-6 w-6 flex-none justify-center px-0 disabled:opacity-50'

// SideChatPane is a side chat's tab: a strip with its title and the ways to add it to its thread's next message, to
// delete it and to close its tab, over its chat, which shows only the timeline and the composer.
export function SideChatPane({ id }: SideChatPaneProps) {
  const side = useOrchestratorSelector((a) => a.sideChats.find((s) => s.id === id))
  const { deleteSideChat, focus, pushToast } = useOrchestrator('deleteSideChat', 'focus', 'pushToast')
  const { closePanelTab } = useLayout()
  const sideChat = useSideChat()
  const [asked, setAsked] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const { anchor, tipHandlers } = useDelayedTip(300)
  const canAdd = side !== undefined && asked

  const addToChat = () => {
    if (!side || !canAdd) return
    attach(side.forkOf, side.id)
    focus('composer')
  }

  const remove = async () => {
    if (!side || deleting) return
    const { ok } = await askConfirm({ key: `delete-side-chat ${id}`, title: `Delete side chat "${side.title}"?`, body: ['This permanently clears its conversation history.'], confirm: 'Delete', tone: 'danger' })
    if (!ok) return
    setDeleting(true)
    try {
      await deleteSideChat(id, false)
      detach(side.forkOf, id)
      closePanelTab(sideChatTab(id))
    } catch (err) {
      pushToast({ tone: 'error', text: 'Could not delete the side chat', detail: errorMessage(err) })
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div data-side-chat-pane={id} className="absolute inset-0 flex flex-col">
      <div className="flex h-9 flex-none items-center gap-1 border-b border-line pr-1 pl-3">
        <h2 className="min-w-0 flex-1 truncate text-small font-medium">{side?.title ?? SIDE_CHAT_TITLE}</h2>
        <button className="btn btn-ghost h-6 flex-none px-2 aria-disabled:cursor-default aria-disabled:opacity-50" aria-disabled={!canAdd} onClick={addToChat} {...tipHandlers}>
          Add to chat
        </button>
        {anchor && <FloatingTip anchor={anchor} text={canAdd ? "Adds this side chat to the main thread's next message." : 'Available after the first message.'} below />}
        <button className={iconButton} title="Delete side chat" aria-label="Delete side chat" disabled={!side || deleting} onClick={() => void remove()}>
          <Trash2 size={14} aria-hidden="true" />
        </button>
        <button className={iconButton} title="Close side chat" aria-label="Close side chat" onClick={() => sideChat.close(id)}>
          <X size={14} aria-hidden="true" />
        </button>
      </div>
      {side ? (
        <div className="relative min-h-0 flex-1">
          <ChatView thread={side} embedded onAsked={setAsked} />
        </div>
      ) : (
        <p className="p-3 text-small text-soft">This side chat was deleted.</p>
      )}
    </div>
  )
}
