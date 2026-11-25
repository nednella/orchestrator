import { memo } from 'react'
import { api } from '../api'
import { ChatMessageActions } from './ChatMessageActions'
import { NoteImages } from './NoteImages'

// A fork from the message continues from the reply before it, forkAt, with the message's text as its draft.
type ChatUserRowProps = { thread: string; text: string; files: string[]; forkAt: string; onReply(text: string): void; onFork(at: string, draft: string): void; onOpenImage(url: string): void }

export const ChatUserRow = memo(function ChatUserRow({ thread, text, files, forkAt, onReply, onFork, onOpenImage }: ChatUserRowProps) {
  const urls = files.flatMap((file) => api.chatMediaURL(thread, file) ?? [])
  return (
    <div data-message className="group/message relative -mb-3 flex flex-col items-end gap-2 pb-6">
      {urls.length > 0 && (
        <div className="flex max-w-[85%] justify-end">
          <NoteImages urls={urls} attachedTo="message" onOpen={onOpenImage} />
        </div>
      )}
      {text && <p className="max-w-[85%] rounded-lg bg-raised px-3.5 py-2 text-body leading-[1.55] break-words whitespace-pre-wrap select-text">{text}</p>}
      {text && <ChatMessageActions text={text} onReply={onReply} onFork={forkAt ? () => onFork(forkAt, text) : undefined} align="end" />}
    </div>
  )
})
