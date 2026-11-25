import { memo } from 'react'
import { useAgentos } from '../AgentosContext'
import { toolSummary } from '../chatTools'
import { useAnswering } from '../useAnswering'
import { ChatAllowDeny } from './ChatAllowDeny'
import { ChatNotice } from './ChatNotice'
import { ChatToolInput } from './ChatToolInput'
import { Icon } from './Icon'

type ChatApprovalCardProps = { thread: string; request: string; tool: string; input: unknown; expired: boolean; folder: string }

export const ChatApprovalCard = memo(function ChatApprovalCard({ thread, request, tool, input, expired, folder }: ChatApprovalCardProps) {
  if (expired) return <ChatNotice text={`The approval to use ${tool} expired`} error={false} />
  return <WaitingApproval thread={thread} request={request} tool={tool} input={input} folder={folder} />
})

type WaitingApprovalProps = { thread: string; request: string; tool: string; input: unknown; folder: string }

function WaitingApproval({ thread, request, tool, input, folder }: WaitingApprovalProps) {
  const { answer } = useAgentos('answer')
  const { busy, send } = useAnswering()
  const { icon, title, detail } = toolSummary(tool, input, folder)

  return (
    <div className="chat-card" data-waiting="true">
      <div className="flex min-w-0 items-center gap-2 text-small">
        <span className="flex flex-none text-dim">
          <Icon name={icon} size={12} />
        </span>
        <span className="flex-none font-medium text-ink">{title}</span>
        {detail && <span className="mono min-w-0 truncate text-dim">{detail}</span>}
      </div>
      <div className="max-h-80 overflow-y-auto select-text">
        <ChatToolInput tool={tool} input={input} />
      </div>
      <ChatAllowDeny
        allow="Allow"
        deny="Deny"
        reasonHint="Tell Claude why (optional)"
        busy={busy}
        onAllow={() => send(() => answer(thread, request, true))}
        onDeny={(reason) => send(() => answer(thread, request, false, reason))}
      />
    </div>
  )
}
