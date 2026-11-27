import { useOrchestrator, useOrchestratorSelector } from '../OrchestratorContext'
import { descendantsOf } from '../threads'
import { ChatApprovalCard } from './ChatApprovalCard'
import { ChatPlanCard } from './ChatPlanCard'
import { ChatQuestionCard } from './ChatQuestionCard'

type ChatChildApprovalsProps = { parent: string }

// ChatChildApprovals is, above the parent's composer, what each of its descendants waits on the owner for, answered
// from here as the descendant's own.
export function ChatChildApprovals({ parent }: ChatChildApprovalsProps) {
  // One string of ids, so the list renders again only when who waits changes.
  const waiting = useOrchestratorSelector((a) =>
    descendantsOf(parent, a.threads)
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
// the descendant's. One whose input is not known, or too large to send, offers no answer, since the owner would not see
// what it lets run: it opens the descendant, whose own chat has the full card. A question's picks are kept by the request, as the pending names no tool call.
function ChildApproval({ id }: ChildApprovalProps) {
  const { select } = useOrchestrator('select')
  const child = useOrchestratorSelector((a) => a.threads.find((s) => s.id === id))
  const pending = useOrchestratorSelector((a) => a.pending[id])
  const projectDir = useOrchestratorSelector((a) => a.projects.find((p) => p.key === child?.project)?.dir ?? '')
  if (!pending || !child) return null
  const known = !pending.inputTooLarge && pending.input !== null && pending.input !== undefined
  return (
    <div className="flex flex-col gap-1">
      <p className="px-1 text-small text-dim">
        <span className="font-medium text-ink">{child.title}</span> asks
      </p>
      {known && pending.kind === 'tool' && <ChatApprovalCard key={pending.request} thread={id} request={pending.request} tool={pending.tool} input={pending.input} expired={false} folder={child.worktree || projectDir} />}
      {known && pending.kind === 'question' && <ChatQuestionCard key={pending.request} thread={id} id={pending.request} request={pending.request} input={pending.input} answer={undefined} answers={null} expired={false} result={null} />}
      {known && pending.kind === 'plan' && <ChatPlanCard key={pending.request} thread={id} request={pending.request} input={pending.input} answer={undefined} expired={false} result={null} />}
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
