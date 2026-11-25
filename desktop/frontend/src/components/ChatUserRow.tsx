import { memo } from 'react'

type ChatUserRowProps = { text: string }

export const ChatUserRow = memo(function ChatUserRow({ text }: ChatUserRowProps) {
  return (
    <div className="flex justify-end">
      <p className="max-w-[85%] rounded-lg bg-raised px-3.5 py-2 text-body leading-[1.55] break-words whitespace-pre-wrap select-text">{text}</p>
    </div>
  )
})
