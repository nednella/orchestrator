import { useEffect, useState } from 'react'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { api } from '../api'
import { approvalInput } from '../childCards'
import { descendantsOf } from '../threads'
import { ChatApprovalCard } from './ChatApprovalCard'

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

type Asked = { request: string; input: unknown }

function ChildApproval({ id }: ChildApprovalProps) {
  const { select } = useAgentos('select')
  const child = useAgentosSelector((a) => a.sessions.find((s) => s.id === id))
  const pending = useAgentosSelector((a) => a.pending[id])
  const projectDir = useAgentosSelector((a) => a.projects.find((p) => p.key === child?.project)?.dir ?? '')
  const [asked, setAsked] = useState<Asked | null>(null)
  const request = pending?.kind === 'tool' ? pending.request : ''

  // The pending approval names its tool but not its input, which the child's log holds.
  useEffect(() => {
    if (!request) return
    let current = true
    api.chatHistory(id).then(
      (events) => current && setAsked({ request, input: approvalInput(events, request) ?? {} }),
      () => current && setAsked({ request, input: {} }),
    )
    return () => {
      current = false
    }
  }, [id, request])

  if (!pending || !child) return null
  const heading = (
    <p className="px-1 text-small text-dim">
      <span className="font-medium text-ink">{child.title}</span> asks
    </p>
  )
  // A question or a plan is answered in the child's own chat, where its card is.
  if (pending.kind !== 'tool')
    return (
      <div className="flex flex-col gap-1">
        {heading}
        <div className="chat-card flex-row items-center gap-2 text-small" data-waiting="true">
          <span className="min-w-0 truncate text-ink">{pending.summary}</span>
          <button className="ml-auto flex-none text-dim hover:text-ink" onClick={() => select(id)}>
            Open
          </button>
        </div>
      </div>
    )
  if (asked?.request !== pending.request) return null
  return (
    <div className="flex flex-col gap-1">
      {heading}
      <ChatApprovalCard thread={id} request={pending.request} tool={pending.tool} input={asked.input} expired={false} folder={child.worktree || projectDir} />
    </div>
  )
}
