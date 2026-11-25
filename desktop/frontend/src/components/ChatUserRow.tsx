import { Pencil } from 'lucide-react'
import { memo } from 'react'
import { api } from '../api'
import { ChatActionButton, revealOnHover } from './ChatActionButton'
import { ChatCopyButton } from './ChatCopyButton'
import { ChatMessageTime } from './ChatMessageTime'
import { NoteImages } from './NoteImages'

// afterUser and beforeUser say the owner's message comes right after or right before another of the owner's, which
// it then stacks with. Edit shows only with onEditFrom, and waits while the agent is working.
type ChatUserRowProps = {
  thread: string
  id: string
  text: string
  files: string[]
  at: number
  afterUser: boolean
  beforeUser: boolean
  working?: boolean
  onEditFrom?(eventId: string): void
  onOpenImage(url: string): void
}

export const ChatUserRow = memo(function ChatUserRow({ thread, id, text, files, at, afterUser, beforeUser, working = false, onEditFrom, onOpenImage }: ChatUserRowProps) {
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
      <div className={`flex w-full flex-col items-end gap-1 ${side}`}>
        {text && <p className={`max-w-[80%] rounded-2xl bg-[var(--bg-user-message)] p-3 text-body leading-[1.55] break-words whitespace-pre-wrap select-text ${corners}`}>{text}</p>}
        <div className={`flex max-w-[80%] flex-none items-center justify-end gap-2 pr-1 select-none ${revealOnHover}`}>
          <ChatMessageTime at={at} />
          <div className="flex items-center gap-0.5">
            {onEditFrom && (
              <ChatActionButton label="Edit from here" disabled={working} onClick={() => onEditFrom(id)}>
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
