import type { CSSProperties } from 'react'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { useLayout } from '../LayoutContext'

const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

type TitleBreadcrumbProps = { showsThread: boolean }

export function TitleBreadcrumb({ showsThread }: TitleBreadcrumbProps) {
  const { project, projects, showHome, select } = useAgentos('project', 'projects', 'showHome', 'select')
  const thread = useAgentosSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  const parent = useAgentosSelector((a) => (thread?.parent ? a.sessions.find((s) => s.id === thread.parent) : undefined))
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
  return (
    <nav aria-label="Breadcrumb" className="flex min-w-0 items-center gap-2 text-body">
      <button
        className="max-w-[12rem] min-w-8 shrink truncate rounded-sm text-soft hover:text-ink"
        style={noDrag}
        title={`Show ${owner} home`}
        onClick={() => showHome(thread.project)}
      >
        {owner}
      </button>
      <span className="flex-none text-dim" aria-hidden="true">
        /
      </span>
      {parent && (
        <>
          <button className="max-w-[12rem] min-w-8 shrink truncate rounded-sm text-soft hover:text-ink" style={noDrag} title={`Open ${parent.title}`} onClick={() => select(parent.id)}>
            {parent.title}
          </button>
          <span className="flex-none text-dim" aria-hidden="true">
            /
          </span>
        </>
      )}
      <span className="min-w-0 truncate" aria-current="page" title={thread.title}>
        {thread.title}
      </span>
    </nav>
  )
}
