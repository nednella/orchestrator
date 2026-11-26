import { useOrchestrator, useOrchestratorSelector } from '../OrchestratorContext'
import { useThreadAgents } from '../chatAgents'
import { viewedAgent } from '../navHistory'
import { ThreadBreadcrumb } from './ThreadBreadcrumb'

export function TitleBreadcrumb() {
  const { project, projects, showHome, select } = useOrchestrator('project', 'projects', 'showHome', 'select')
  const thread = useOrchestratorSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const parent = useOrchestratorSelector((a) => (thread?.parent ? a.sessions.find((s) => s.id === thread.parent) : undefined))
  const viewed = useOrchestratorSelector((a) => viewedAgent(a.viewHistory, thread?.id ?? ''))
  const agent = useThreadAgents(thread?.id ?? '').find((a) => a.id === viewed)

  if (!thread) {
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
