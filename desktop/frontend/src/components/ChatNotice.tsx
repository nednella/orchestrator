import { memo } from 'react'

type ChatNoticeProps = { text: string; error: boolean }

export const ChatNotice = memo(function ChatNotice({ text, error }: ChatNoticeProps) {
  return <p className={`text-center text-small ${error ? 'text-danger' : 'text-dim'}`}>{text}</p>
})
