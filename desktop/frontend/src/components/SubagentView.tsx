import { memo, useEffect, useRef, useState } from 'react'
import type { CSSProperties } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { useShowAgents } from '../chatAgents'
import { COLUMN_KEY, storedColumn } from '../chatColumn'
import { sameSteps } from '../chatTimeline'
import type { AgentItem, AgentState, WorkStep } from '../chatTimeline'
import { readStored } from '../storage'
import type { Thread } from '../types'
import { useChat } from '../useChat'
import { useFollowEnd } from '../useFollowEnd'
import { ChatLineageDivider } from './ChatLineageDivider'
import { ChatMarkdown } from './ChatMarkdown'
import { ChatSteps } from './ChatSteps'
import { ChatUserText } from './ChatUserText'
import { CenteredSpinner } from './CenteredSpinner'
import { SubagentBar } from './SubagentBar'

type SubagentViewProps = { parent: Thread; agent: string }

// SubagentView shows a subagent Claude started in the parent's chat, read only: what it was asked, what it did and
// what it reported. It follows the parent's chat, which holds every event of the subagent.
export function SubagentView({ parent, agent }: SubagentViewProps) {
  const { project, select, closeGoneAgent } = useOrchestrator('project', 'select', 'closeGoneAgent')
  const { timeline, loading } = useChat(parent.id, parent.state)
  useShowAgents(parent.id, timeline.items)
  const [column] = useState(() => storedColumn(readStored(COLUMN_KEY, null)))
  const found = timeline.items.find((item): item is AgentItem => item.kind === 'agent' && item.id === agent)
  const holds = found !== undefined
  useEffect(() => closeGoneAgent(parent.id, !loading, holds), [closeGoneAgent, parent.id, loading, holds])
  const scroller = useRef<HTMLDivElement>(null)
  const content = useRef<HTMLDivElement>(null)
  const onScroll = useFollowEnd(scroller, content)
  const back = () => select(parent.id)

  return (
    <div data-chat className="relative flex h-full min-h-0 flex-col bg-app" style={{ '--chat-column': `${column}rem` } as CSSProperties}>
      <div ref={scroller} onScroll={onScroll} data-chat-scroll tabIndex={-1} className="chat-scroll min-h-0 flex-1 overflow-y-auto pt-6 outline-none">
        <div ref={content} className="chat-column flex flex-col gap-4 pb-6">
          <ChatLineageDivider parent={parent.title} onOpenParent={back} />
          {found && <SubagentRun thread={parent.id} folder={parent.worktree || project?.dir || ''} id={found.key} prompt={found.prompt} state={found.state} steps={found.steps} result={found.result} />}
          {!found && loading && <CenteredSpinner />}
        </div>
      </div>
      {found && (
        <div className="chat-over flex-none pt-1.5 pb-4 sm:pt-2 sm:pb-5">
          <div className="chat-column">
            <SubagentBar agent={found} parentModel={parent.model} onOpenParent={back} />
          </div>
        </div>
      )}
    </div>
  )
}

type SubagentRunProps = { thread: string; folder: string; id: string; prompt: string; state: AgentState; steps: WorkStep[]; result: string | null }

// The parent's timeline is built anew on every event, so the subagent's steps are new objects holding the same values,
// and an event of the parent or of another subagent leaves the run as it was.
const unchanged = (a: SubagentRunProps, b: SubagentRunProps) => a.thread === b.thread && a.folder === b.folder && a.id === b.id && a.prompt === b.prompt && a.state === b.state && a.result === b.result && sameSteps(a.steps, b.steps)

// SubagentRun is the subagent's prompt, as the message it was sent, then its steps and its report.
const SubagentRun = memo(function SubagentRun({ thread, folder, id, prompt, state, steps, result }: SubagentRunProps) {
  return (
    <>
      {prompt && (
        <div className="max-w-[80%] self-end rounded-2xl bg-[var(--bg-user-message)] p-3">
          <ChatUserText id={id} text={prompt} />
        </div>
      )}
      <div>
        <ChatSteps thread={thread} steps={steps} running={state === 'running'} folder={folder} />
      </div>
      {result && (
        <div className={`text-body ${state === 'failed' ? 'text-danger' : 'text-ink'}`}>
          <ChatMarkdown text={result} />
        </div>
      )}
    </>
  )
}, unchanged)
