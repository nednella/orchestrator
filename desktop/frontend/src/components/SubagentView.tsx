import { useState } from 'react'
import type { CSSProperties } from 'react'
import { useAgentos } from '../AgentosContext'
import { closeAgent, useShowAgents } from '../chatAgents'
import { COLUMN_KEY, storedColumn } from '../chatColumn'
import type { AgentItem } from '../chatTimeline'
import { readStored } from '../storage'
import type { Session } from '../types'
import { useChat } from '../useChat'
import { ChatLineageDivider } from './ChatLineageDivider'
import { ChatMarkdown } from './ChatMarkdown'
import { ChatSteps } from './ChatSteps'
import { ChatUserText } from './ChatUserText'
import { Loading } from './Loading'
import { Notice } from './Notice'
import { SubagentBar } from './SubagentBar'

type SubagentViewProps = { parent: Session; agent: string }

// SubagentView shows a subagent Claude started in the parent's chat, read only: what it was asked, what it did and
// what it reported. It follows the parent's chat, which holds every event of the subagent.
export function SubagentView({ parent, agent }: SubagentViewProps) {
  const { project } = useAgentos('project')
  const { timeline, loading, error } = useChat(parent.id, parent.state)
  useShowAgents(parent.id, timeline.items)
  const [column] = useState(() => storedColumn(readStored(COLUMN_KEY, null)))
  const found = timeline.items.find((item): item is AgentItem => item.kind === 'agent' && item.id === agent)
  const back = () => closeAgent(parent.id)

  return (
    <div data-chat className="relative flex h-full min-h-0 flex-col bg-app" style={{ '--chat-column': `${column}rem` } as CSSProperties}>
      <div data-chat-scroll tabIndex={-1} className="chat-scroll min-h-0 flex-1 overflow-y-auto pt-6 outline-none">
        <div className="chat-column flex flex-col gap-4 pb-6">
          <ChatLineageDivider parent={parent.title} onOpenParent={back} />
          {found && <SubagentRun agent={found} thread={parent.id} folder={parent.worktree || project?.dir || ''} />}
          {!found && loading && <Loading />}
          {!found && !loading && <Notice title="This subagent is not in the chat" hint={error || 'Its thread no longer holds it.'} centered />}
        </div>
      </div>
      {found && (
        <div className="chat-over flex-none pt-1.5 pb-4 sm:pt-2 sm:pb-5">
          <div className="chat-column">
            <SubagentBar agent={found} onOpenParent={back} />
          </div>
        </div>
      )}
    </div>
  )
}

type SubagentRunProps = { agent: AgentItem; thread: string; folder: string }

// SubagentRun is the subagent's prompt, as the message it was sent, then its steps and its report.
function SubagentRun({ agent, thread, folder }: SubagentRunProps) {
  return (
    <>
      {agent.prompt && (
        <div className="max-w-[80%] self-end rounded-2xl bg-[var(--bg-user-message)] p-3">
          <ChatUserText id={agent.key} text={agent.prompt} />
        </div>
      )}
      <div>
        <ChatSteps thread={thread} steps={agent.steps} running={agent.state === 'running'} folder={folder} />
      </div>
      {agent.result && (
        <div className={`text-body ${agent.state === 'failed' ? 'text-danger' : 'text-ink'}`}>
          <ChatMarkdown text={agent.result} />
        </div>
      )}
    </>
  )
}
