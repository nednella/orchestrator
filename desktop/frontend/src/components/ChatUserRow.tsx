import { Pencil } from 'lucide-react'
import { memo } from 'react'
import { useOrchestratorSelector } from '../OrchestratorContext'
import { api } from '../api'
import { ChatActionButton, revealOnHover } from './ChatActionButton'
import { ChatCopyButton } from './ChatCopyButton'
import { ChatMessageTime } from './ChatMessageTime'
import { ChatUserText } from './ChatUserText'
import { NoteImages } from './NoteImages'

// afterUser and beforeUser say the owner's message comes right after or right before another of the owner's, which
// it then stacks with. uuid is the message's in Claude's conversation. Edit shows only with onEditFrom, and is off
// while editBlocked says why. from is the thread that sent the message, missing for the owner's.
type ChatUserRowProps = {
  thread: string
  id: string
  uuid: string
  text: string
  files: string[]
  from?: string
  at: number
  afterUser: boolean
  beforeUser: boolean
  editBlocked?: string
  onEditFrom?(uuid: string): void
  onOpenImage(url: string): void
}

export const ChatUserRow = memo(function ChatUserRow({ thread, id, uuid, text, files, from, at, afterUser, beforeUser, editBlocked = '', onEditFrom, onOpenImage }: ChatUserRowProps) {
  const urls = files.flatMap((file) => api.chatMediaURL(thread, file) ?? [])
  // Stacked messages keep their actions to the side of the message, where the column is wide enough.
  const side = beforeUser ? '@min-[37rem]:flex-row-reverse @min-[37rem]:items-center @min-[37rem]:gap-2' : ''
  const corners = `${afterUser ? 'rounded-tr-md' : ''} ${beforeUser ? 'rounded-br-md' : ''}`
  return (
    <div data-message className={`group/message @container flex flex-col items-end gap-1 ${beforeUser ? '-mb-4' : '-mb-3'}`}>
      {urls.length > 0 && (
        <div className="flex max-w-[80%] justify-end">
          <NoteImages urls={urls} attachedTo="message" onOpen={onOpenImage} />
        </div>
      )}
      {from && <Sender id={from} />}
      <div className={`flex w-full flex-col items-end gap-1 ${side}`}>
        {text && (
          <div className={`max-w-[80%] rounded-2xl bg-[var(--bg-user-message)] p-3 ${corners}`}>
            <ChatUserText id={id} text={text} />
          </div>
        )}
        <div className={`flex max-w-[80%] flex-none items-center justify-end gap-2 pr-1 select-none ${revealOnHover}`}>
          <ChatMessageTime at={at} />
          <div className="flex items-center gap-0.5">
            {onEditFrom && (
              <ChatActionButton label={editBlocked || 'Edit from here'} disabled={editBlocked !== ''} onClick={() => onEditFrom(uuid)}>
                <Pencil className="size-3" />
              </ChatActionButton>
            )}
            {text && <ChatCopyButton text={text} />}
          </div>
        </div>
      </div>
    </div>
  )
})

type SenderProps = { id: string }

// Sender heads a message another thread sent, by that thread's title while the sessions list holds it.
function Sender({ id }: SenderProps) {
  const title = useOrchestratorSelector((a) => a.sessions.find((s) => s.id === id)?.title ?? id)
  return <p className="max-w-[80%] truncate pr-1 text-small text-dim">From {title}</p>
}
