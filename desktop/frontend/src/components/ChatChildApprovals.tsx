import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { descendantsOf } from '../threads'
import { ChatApprovalCard } from './ChatApprovalCard'
import { ChatPlanCard } from './ChatPlanCard'
import { ChatQuestionCard } from './ChatQuestionCard'

type ChatChildApprovalsProps = { parent: string }

// ChatChildApprovals is, above the parent's composer, what each of its descendants waits on the owner for, answered
// from here as the descendant's own.
export function ChatChildApprovals({ parent }: ChatChildApprovalsProps) {
  // One string of ids, so the list renders again only when who waits changes.
  const waiting = useAgentosSelector((a) =>
    descendantsOf(parent, a.sessions)
      .filter((d) => a.pending[d.id])
      .map((d) => d.id)
      .join(' '),
  )
  if (!waiting) return null
  return (
    <div className="glass mb-2 flex max-h-[50vh] flex-col gap-2 overflow-y-auto rounded-2xl p-2">
      {waiting.split(' ').map((id) => (
        <ChildApproval key={id} id={id} />
      ))}
    </div>
  )
}

type ChildApprovalProps = { id: string }

// ChildApproval is the approval one descendant waits on, on the card its own chat shows for it, answered from here as
// the descendant's. One whose input is not known offers no answer, since the owner would not see what it lets run: it
// opens the descendant instead. A question's picks are kept by the request, as the pending names no tool call.
function ChildApproval({ id }: ChildApprovalProps) {
  const { select } = useAgentos('select')
  const child = useAgentosSelector((a) => a.sessions.find((s) => s.id === id))
  const pending = useAgentosSelector((a) => a.pending[id])
  const projectDir = useAgentosSelector((a) => a.projects.find((p) => p.key === child?.project)?.dir ?? '')
  if (!pending || !child) return null
  const known = pending.input !== null && pending.input !== undefined
  return (
    <div className="flex flex-col gap-1">
      <p className="px-1 text-small text-dim">
        <span className="font-medium text-ink">{child.title}</span> asks
      </p>
      {known && pending.kind === 'tool' && <ChatApprovalCard thread={id} request={pending.request} tool={pending.tool} input={pending.input} expired={false} folder={child.worktree || projectDir} />}
      {known && pending.kind === 'question' && <ChatQuestionCard thread={id} id={pending.request} request={pending.request} input={pending.input} answer={undefined} answers={null} expired={false} result={null} />}
      {known && pending.kind === 'plan' && <ChatPlanCard thread={id} request={pending.request} input={pending.input} answer={undefined} expired={false} result={null} />}
      {!known && (
        <div className="chat-card flex-row items-center gap-2 text-small" data-waiting="true">
          <span className="min-w-0 truncate text-ink">{pending.summary}</span>
          <button className="ml-auto flex-none text-dim hover:text-ink" onClick={() => select(id)}>
            Open
          </button>
        </div>
      )}
    </div>
  )
}
