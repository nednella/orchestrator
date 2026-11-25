import { memo } from 'react'
import { useAgentos } from '../AgentosContext'
import { useAnswering } from '../useAnswering'
import { ChatAllowDeny } from './ChatAllowDeny'
import { ChatMarkdown } from './ChatMarkdown'

// request is '' until Claude asks for the approval that answers the plan.
type ChatPlanCardProps = { thread: string; request: string; input: unknown; answer: 'allow' | 'deny' | undefined; expired: boolean; result: string | null }

const planOf = (input: unknown): string => {
  const plan = (input as { plan?: unknown } | null)?.plan
  return typeof plan === 'string' ? plan : ''
}

export const ChatPlanCard = memo(function ChatPlanCard({ thread, request, input, answer, expired, result }: ChatPlanCardProps) {
  const open = !answer && !expired && result === null
  const status = answer === 'allow' ? 'Plan approved' : answer === 'deny' ? 'Kept planning' : expired ? 'The plan expired' : ''

  return (
    <div className="chat-card" data-waiting={open}>
      <p className="label">Plan</p>
      <ChatMarkdown text={planOf(input)} />
      {open && <PlanAnswer thread={thread} request={request} />}
      {status && <p className="text-small text-dim">{status}</p>}
    </div>
  )
})

type PlanAnswerProps = { thread: string; request: string }

function PlanAnswer({ thread, request }: PlanAnswerProps) {
  const a = useAgentos('answer')
  const { busy, send } = useAnswering()
  return (
    <ChatAllowDeny
      allow="Approve plan"
      deny="Keep planning"
      reasonHint="What should change? (optional)"
      busy={busy || request === ''}
      onAllow={() => send(() => a.answer(thread, request, true))}
      onDeny={(reason) => send(() => a.answer(thread, request, false, reason))}
    />
  )
}
