import { useOrchestrator, useOrchestratorSelector } from '../OrchestratorContext'
import { useThreadAgents } from '../chatAgents'
import { useLayout } from '../LayoutContext'
import { viewedAgent } from '../navHistory'
import { ThreadBreadcrumb } from './ThreadBreadcrumb'

type TitleBreadcrumbProps = { showsThread: boolean }

export function TitleBreadcrumb({ showsThread }: TitleBreadcrumbProps) {
  const { project, projects, showHome, select } = useOrchestrator('project', 'projects', 'showHome', 'select')
  const thread = useOrchestratorSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const parent = useOrchestratorSelector((a) => (thread?.parent ? a.sessions.find((s) => s.id === thread.parent) : undefined))
  const viewed = useOrchestratorSelector((a) => viewedAgent(a.viewHistory, thread?.id ?? ''))
  const agent = useThreadAgents(thread?.id ?? '').find((a) => a.id === viewed)
  const { digestOpen } = useLayout()

  if (digestOpen) {
    return (
      <nav aria-label="Breadcrumb" className="flex min-w-0 items-center gap-2 text-body">
        <span className="max-w-[12rem] min-w-8 shrink truncate text-soft">{project?.name}</span>
        <span className="flex-none text-dim" aria-hidden="true">
          /
        </span>
        <span className="min-w-0 truncate" aria-current="page">
          News
        </span>
      </nav>
    )
  }

  if (!thread || !showsThread) {
    return <span className="min-w-0 truncate text-body">{project?.name}</span>
  }

  const owner = projects.find((p) => p.key === thread.project)?.name ?? thread.project
  // Above a subagent's view is its thread; above a thread, the thread that delegated to it. A grandchild shows only its
  // own parent.
  const above = agent ? thread : parent
  return (
    <ThreadBreadcrumb
      project={owner}
      onShowProject={() => showHome(thread.project)}
      parent={above ? above.title : null}
      onOpenParent={() => above && select(above.id)}
      title={agent ? agent.description : thread.title}
    />
  )
}
