import { Bot, CircleDot, CornerLeftUp, GitBranch, GitFork, GitMerge, GitPullRequestArrow, GitPullRequestClosed, GitPullRequestDraft, Globe, Laptop, MessagesSquare } from 'lucide-react'
import type { MouseEvent } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'
import { splitBranch } from '../branchLabel'
import { editorLabel } from '../contextCard'
import type { CardRow } from '../contextCard'
import { closeCardPopover } from '../contextCardOpen'
import { askToReadIssue } from '../issueReadRequest'
import { useLayout } from '../LayoutContext'
import type { PanelViewId } from '../panelTabs'
import { AGENT_STATE_LABEL, STATE_LABEL } from '../stateMeta'
import type { PR } from '../types'
import { useCardFacts } from '../useCardFacts'
import { useCopied } from '../useCopied'
import { useSideChat } from '../useSideChat'
import { ChecksMark } from './ChecksMark'
import { ClaudeMark } from './ClaudeMark'
import { ContextCardRow } from './ContextCardRow'
import { EditorMark } from './EditorMark'
import { ServerMeta } from './ServerMeta'

const PR_ICON = { open: GitPullRequestArrow, draft: GitPullRequestDraft, closed: GitPullRequestClosed, merged: GitMerge }
const PR_STATE: Record<PR['state'], string> = { open: 'Open', draft: 'Draft', closed: 'Closed', merged: 'Merged' }
const CHECKS: Record<PR['checks'], string> = { none: '', pending: 'Checks running', passing: 'Checks passed', failing: 'Checks failed' }
const RELATION_ICON = { parent: CornerLeftUp, child: Bot, fork: GitFork }

type ThreadContextCardProps = { id: string; className: string }

// ThreadContextCard is what the thread works with, at a glance: where it runs, its branch, PR and issue, its servers
// and the threads and subagents around it, and its side chats.
export function ThreadContextCard({ id, className }: ThreadContextCardProps) {
  const a = useOrchestrator('select', 'report', 'openInEditor', 'openAgent')
  const { showPanelView, focusPanel } = useLayout()
  const sideChat = useSideChat()
  const { session, sections, refreshServers } = useCardFacts(id)
  const [copied, copyBranch] = useCopied(session?.branch ?? '')
  if (!session || sections.length === 0) return null
  const show = (view: PanelViewId) => {
    closeCardPopover()
    showPanelView(view)
  }
  const copy = (e: MouseEvent) => {
    e.preventDefault()
    copyBranch()
  }

  const row = (r: CardRow) => {
    switch (r.kind) {
      case 'workspace':
        return <ContextCardRow icon={<Laptop size={16} aria-hidden="true" />} label={r.machine} tip={r.folder} />
      case 'editor':
        return <ContextCardRow icon={<EditorMark editor={r.editor} />} label={editorLabel(r.editor)} onClick={() => a.report(() => a.openInEditor(session.id))} />
      case 'branch': {
        const { head, tail } = splitBranch(r.branch)
        const label = (
          <span className="flex min-w-0">
            <span className="truncate">{head}</span>
            {tail && <span className="flex-none">{tail}</span>}
          </span>
        )
        return <ContextCardRow icon={<GitBranch size={16} aria-hidden="true" />} label={label} end={copied && 'Copied'} title={`${r.branch}\nRight-click to copy`} onContextMenu={copy} />
      }
      case 'pr': {
        const PRIcon = PR_ICON[r.pr.state]
        const meta = (
          <>
            {PR_STATE[r.pr.state]}
            {r.pr.checks !== 'none' && (
              <>
                <span aria-hidden="true">·</span>
                <span aria-hidden="true" className="flex">
                  <ChecksMark checks={r.pr.checks} />
                </span>
                {CHECKS[r.pr.checks]}
              </>
            )}
          </>
        )
        return <ContextCardRow icon={<PRIcon size={16} color={`var(--pr-${r.pr.state})`} aria-hidden="true" />} label={`#${r.pr.number}: ${r.pr.title}`} wrap meta={meta} onClick={() => show('pulls')} />
      }
      case 'issue':
        return (
          <ContextCardRow
            icon={<CircleDot size={16} color="var(--pr-open)" aria-hidden="true" />}
            label={r.title ? `#${r.number}: ${r.title}` : `#${r.number}`}
            wrap
            onClick={() => {
              askToReadIssue(r.number)
              show('issues')
            }}
          />
        )
      case 'server': {
        const { port, pid, command } = r.server
        const stop = { label: `Stop the server on port ${port}`, run: () => a.report(() => api.stopServer(session.id, pid).then(refreshServers)) }
        return <ContextCardRow icon={<Globe size={16} aria-hidden="true" />} label={`localhost:${port}`} meta={<ServerMeta server={r.server} />} title={command} onClick={() => a.report(() => api.openServer(port))} stop={stop} />
      }
      case 'agent': {
        const end = <span className={r.agent.state === 'failed' ? 'text-danger' : undefined}>{AGENT_STATE_LABEL[r.agent.state]}</span>
        const open = () => {
          a.openAgent(session.id, r.agent.id)
          focusPanel('centre')
        }
        return <ContextCardRow icon={<ClaudeMark size={16} />} label={r.agent.description} end={end} onClick={open} />
      }
      case 'thread': {
        const RelationIcon = RELATION_ICON[r.relation]
        const dot = r.relation === 'child' ? r.thread.state : undefined
        return <ContextCardRow icon={<RelationIcon size={16} aria-hidden="true" />} dot={dot} label={r.thread.title} end={STATE_LABEL[r.thread.state]} onClick={() => a.select(r.thread.id)} />
      }
      case 'sideChat': {
        const open = () => {
          closeCardPopover()
          sideChat.open(session.id, r.thread.id)
        }
        return <ContextCardRow icon={<MessagesSquare size={16} aria-hidden="true" />} dot={r.thread.state} label={r.thread.title} end={STATE_LABEL[r.thread.state]} title="Open side chat" onClick={open} />
      }
    }
  }

  return (
    <aside aria-label="Thread details" className={`flex w-[280px] flex-col overflow-y-auto rounded-2xl border border-(--border-glass) bg-(--bg-dropdown) bg-(image:--bg-dropdown-tint) shadow-(--shadow-card) backdrop-blur-[16px] backdrop-saturate-[1.08] ${className}`}>
      {sections.map((section) => (
        <section key={section.key} className="flex-none border-b border-(--border-card-section) px-2 pt-2 pb-2.5 last:border-b-0">
          {section.heading && <h3 className="flex min-h-8 items-center px-2.5 text-label font-medium text-soft">{section.heading}</h3>}
          {section.rows.map((r) => (
            <div key={r.key}>{row(r)}</div>
          ))}
        </section>
      ))}
    </aside>
  )
}
